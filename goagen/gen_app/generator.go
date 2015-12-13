package genapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphael/goa/design"
	"github.com/raphael/goa/goagen/codegen"
	"github.com/raphael/goa/goagen/utils"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Generator is the application code generator.
type Generator struct {
	*codegen.GoGenerator
	genfiles []string
}

// Generate is the generator entry point called by the meta generator.
func Generate(api *design.APIDefinition) ([]string, error) {
	g, err := NewGenerator()
	if err != nil {
		return nil, err
	}
	return g.Generate(api)
}

// NewGenerator returns the application code generator.
func NewGenerator() (*Generator, error) {
	app := kingpin.New("Code generator", "application code generator")
	codegen.RegisterFlags(app)
	NewCommand().RegisterFlags(app)
	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return nil, fmt.Errorf(`invalid command line: %s. Command line was "%s"`,
			err, strings.Join(os.Args, " "))
	}
	outdir := AppOutputDir()
	os.RemoveAll(outdir)
	if err = os.MkdirAll(outdir, 0777); err != nil {
		return nil, err
	}
	return &Generator{
		GoGenerator: codegen.NewGoGenerator(outdir),
		genfiles:    []string{outdir},
	}, nil
}

// AppOutputDir returns the directory containing the generated files.
func AppOutputDir() string {
	return filepath.Join(codegen.OutputDir, TargetPackage)
}

// Generate the application code, implement codegen.Generator.
func (g *Generator) Generate(api *design.APIDefinition) (_ []string, err error) {
	go utils.Catch(nil, func() { g.Cleanup() })

	defer func() {
		if err != nil {
			g.Cleanup()
		}
	}()

	if api == nil {
		return nil, fmt.Errorf("missing API definition, make sure design.Design is properly initialized")
	}
	outdir := AppOutputDir()
	err = api.IterateVersions(func(v *design.APIVersionDefinition) error {
		verdir := filepath.Join(outdir, v.Version)
		ctxFile := filepath.Join(verdir, "contexts.go")
		ctlFile := filepath.Join(verdir, "controllers.go")
		resFile := filepath.Join(verdir, "hrefs.go")

		ctxWr, err := NewContextsWriter(ctxFile)
		if err != nil {
			panic(err) // bug
		}
		ctlWr, err := NewControllersWriter(ctlFile)
		if err != nil {
			panic(err) // bug
		}
		resWr, err := NewResourcesWriter(resFile)
		if err != nil {
			panic(err) // bug
		}
		title := fmt.Sprintf("%s: Application Contexts", v.Context())
		imports := []*codegen.ImportSpec{
			codegen.SimpleImport("fmt"),
			codegen.SimpleImport("strconv"),
			codegen.SimpleImport("github.com/raphael/goa"),
		}
		ctxWr.WriteHeader(title, TargetPackage, imports)
		err = api.IterateResources(func(r *design.ResourceDefinition) error {
			return r.IterateActions(func(a *design.ActionDefinition) error {
				ctxName := codegen.Goify(a.Name, true) + codegen.Goify(a.Parent.Name, true) + "Context"
				ctxData := ContextTemplateData{
					Name:         ctxName,
					ResourceName: r.Name,
					ActionName:   a.Name,
					Payload:      a.Payload,
					Params:       a.AllParams(),
					Headers:      r.Headers.Merge(a.Headers),
					Routes:       a.Routes,
					Responses:    MergeResponses(r.Responses, a.Responses),
					API:          api,
				}
				return ctxWr.Execute(&ctxData)
			})
		})
		g.genfiles = append(g.genfiles, ctxFile)
		if err != nil {
			return err
		}
		if err = ctxWr.FormatCode(); err != nil {
			return err
		}

		title = fmt.Sprintf("%s: Application Controllers", v.Context())
		imports = []*codegen.ImportSpec{
			codegen.SimpleImport("github.com/julienschmidt/httprouter"),
			codegen.SimpleImport("github.com/raphael/goa"),
		}
		ctlWr.WriteHeader(title, TargetPackage, imports)
		var controllersData []*ControllerTemplateData
		api.IterateResources(func(r *design.ResourceDefinition) error {
			data := &ControllerTemplateData{Resource: codegen.Goify(r.Name, true)}
			err := r.IterateActions(func(a *design.ActionDefinition) error {
				context := fmt.Sprintf("%s%sContext", codegen.Goify(a.Name, true), codegen.Goify(r.Name, true))
				action := map[string]interface{}{
					"Name":    codegen.Goify(a.Name, true),
					"Routes":  a.Routes,
					"Context": context,
				}
				data.Actions = append(data.Actions, action)
				return nil
			})
			if err != nil {
				return err
			}
			if len(data.Actions) > 0 {
				controllersData = append(controllersData, data)
			}
			return nil
		})
		g.genfiles = append(g.genfiles, ctlFile)
		if err = ctlWr.Execute(controllersData); err != nil {
			return err
		}
		if err = ctlWr.FormatCode(); err != nil {
			return err
		}

		title = fmt.Sprintf("%s: Application Resource Href Factories", v.Context())
		resWr.WriteHeader(title, TargetPackage, nil)
		err = api.IterateResources(func(r *design.ResourceDefinition) error {
			m := api.MediaTypeWithIdentifier(r.MediaType)
			var identifier string
			if m != nil {
				identifier = m.Identifier
			} else {
				identifier = "plain/text"
			}
			canoTemplate := r.URITemplate()
			canoTemplate = design.WildcardRegex.ReplaceAllLiteralString(canoTemplate, "/%v")
			var canoParams []string
			if ca := r.CanonicalAction(); ca != nil {
				if len(ca.Routes) > 0 {
					canoParams = ca.Routes[0].Params()
				}
			}

			data := ResourceData{
				Name:              codegen.Goify(r.Name, true),
				Identifier:        identifier,
				Description:       r.Description,
				Type:              m,
				CanonicalTemplate: canoTemplate,
				CanonicalParams:   canoParams,
			}
			return resWr.Execute(&data)
		})
		g.genfiles = append(g.genfiles, resFile)
		if err != nil {
			return err
		}
		if err = resWr.FormatCode(); err != nil {
			return err
		}
		return nil
	})
	mtFile := filepath.Join(outdir, "media_types.go")
	utFile := filepath.Join(outdir, "user_types.go")
	mtWr, err := NewMediaTypesWriter(mtFile)
	if err != nil {
		panic(err) // bug
	}
	utWr, err := NewUserTypesWriter(utFile)
	if err != nil {
		panic(err) // bug
	}

	title := fmt.Sprintf("%s: Application Media Types", api.Context())
	imports := []*codegen.ImportSpec{
		codegen.SimpleImport("github.com/raphael/goa"),
		codegen.SimpleImport("fmt"),
	}
	mtWr.WriteHeader(title, TargetPackage, imports)
	err = api.IterateMediaTypes(func(mt *design.MediaTypeDefinition) error {
		if mt.Type.IsObject() || mt.Type.IsArray() {
			return mtWr.Execute(mt)
		}
		return nil
	})
	g.genfiles = append(g.genfiles, mtFile)
	if err != nil {
		return
	}
	if err = mtWr.FormatCode(); err != nil {
		return
	}

	title = fmt.Sprintf("%s: Application User Types", api.Context())
	imports = []*codegen.ImportSpec{
		codegen.SimpleImport("github.com/raphael/goa"),
	}
	utWr.WriteHeader(title, TargetPackage, imports)
	err = api.IterateUserTypes(func(t *design.UserTypeDefinition) error {
		return utWr.Execute(t)
	})
	g.genfiles = append(g.genfiles, utFile)
	if err != nil {
		return
	}
	if err = utWr.FormatCode(); err != nil {
		return
	}

	return g.genfiles, nil
}

// Cleanup removes the entire "app" directory if it was created by this generator.
func (g *Generator) Cleanup() {
	if len(g.genfiles) == 0 {
		return
	}
	os.RemoveAll(AppOutputDir())
	g.genfiles = nil
}

// MergeResponses merge the response maps overriding the first argument map entries with the
// second argument map entries in case of collision.
func MergeResponses(l, r map[string]*design.ResponseDefinition) map[string]*design.ResponseDefinition {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	for n, r := range r {
		l[n] = r
	}
	return l
}
