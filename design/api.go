package design

import (
	"fmt"
	"mime"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/goadesign/goa/dslengine"
	"github.com/julienschmidt/httprouter"
)

var (
	// Design is the API definition created via DSL.
	Design *APIDefinition

	// WildcardRegex is the regular expression used to capture path parameters.
	WildcardRegex = regexp.MustCompile(`/(?::|\*)([a-zA-Z0-9_]+)`)

	// GeneratedMediaTypes contains DSL definitions that were created by the design DSL and
	// need to be executed as a second pass.
	// An example of this are media types defined with CollectionOf: the element media type
	// must be defined first then the definition created by CollectionOf must execute.
	GeneratedMediaTypes MediaTypeRoot

	// DefaultDecoders contains the decoding definitions used when no Consumes DSL is found.
	DefaultDecoders []*EncodingDefinition

	// DefaultEncoders contains the encoding definitions used when no Produces DSL is found.
	DefaultEncoders []*EncodingDefinition

	// KnownEncoders contains the list of encoding packages and factories known by goa indexed
	// by MIME type.
	KnownEncoders = map[string][3]string{
		"application/json":      {"json", "JSONEncoderFactory", "JSONDecoderFactory"},
		"application/xml":       {"xml", "XMLEncoderFactory", "XMLDecoderFactory"},
		"text/xml":              {"xml", "XMLEncoderFactory", "XMLDecoderFactory"},
		"application/gob":       {"gob", "GobEncoderFactory", "GobDecoderFactory"},
		"application/x-gob":     {"gob", "GobEncoderFactory", "GobDecoderFactory"},
		"application/binc":      {"github.com/goadesign/encoding/binc", "EncoderFactory", "DecoderFactory"},
		"application/x-binc":    {"github.com/goadesign/encoding/binc", "EncoderFactory", "DecoderFactory"},
		"application/x-cbor":    {"github.com/goadesign/encoding/cbor", "EncoderFactory", "DecoderFactory"},
		"application/cbor":      {"github.com/goadesign/encoding/cbor", "EncoderFactory", "DecoderFactory"},
		"application/msgpack":   {"github.com/goadesign/encoding/msgpack", "EncoderFactory", "DecoderFactory"},
		"application/x-msgpack": {"github.com/goadesign/encoding/msgpack", "EncoderFactory", "DecoderFactory"},
	}

	// JSONContentTypes is a slice of default Content-Type headers that will use stdlib
	// encoding/json to unmarshal unless overwritten using SetDecoder
	JSONContentTypes = []string{"application/json"}

	// XMLContentTypes is a slice of default Content-Type headers that will use stdlib
	// encoding/xml to unmarshal unless overwritten using SetDecoder
	XMLContentTypes = []string{"application/xml", "text/xml"}

	// GobContentTypes is a slice of default Content-Type headers that will use stdlib
	// encoding/gob to unmarshal unless overwritten using SetDecoder
	GobContentTypes = []string{"application/gob", "application/x-gob"}
)

func init() {
	var types []string
	types = append(types, JSONContentTypes...)
	types = append(types, XMLContentTypes...)
	types = append(types, GobContentTypes...)
	DefaultEncoders = []*EncodingDefinition{{MIMETypes: types}}
	DefaultDecoders = []*EncodingDefinition{{MIMETypes: types}}
}

type (
	// APIDefinition defines the global properties of the API.
	APIDefinition struct {
		// APIVersionDefinition contains the default values for properties across all versions.
		*APIVersionDefinition
		// APIVersions contain the API properties indexed by version.
		APIVersions map[string]*APIVersionDefinition
		// Exposed resources indexed by name
		Resources map[string]*ResourceDefinition
		// Types indexes the user defined types by name.
		Types map[string]*UserTypeDefinition
		// MediaTypes indexes the API media types by canonical identifier.
		MediaTypes map[string]*MediaTypeDefinition
		// rand is the random generator used to generate examples.
		rand *RandomGenerator
	}

	// APIVersionDefinition defines the properties of the API for a given version.
	APIVersionDefinition struct {
		// API name
		Name string
		// API Title
		Title string
		// API description
		Description string
		// API version if any
		Version string
		// API hostname
		Host string
		// API URL schemes
		Schemes []string
		// Common base path to all API actions
		BasePath string
		// Common path parameters to all API actions
		BaseParams *AttributeDefinition
		// Consumes lists the mime types supported by the API controllers.
		Consumes []*EncodingDefinition
		// Produces lists the mime types generated by the API controllers.
		Produces []*EncodingDefinition
		// TermsOfService describes or links to the API terms of service
		TermsOfService string
		// Contact provides the API users with contact information
		Contact *ContactDefinition
		// License describes the API license
		License *LicenseDefinition
		// Docs points to the API external documentation
		Docs *DocsDefinition
		// Traits available to all API resources and actions indexed by name
		Traits map[string]*dslengine.TraitDefinition
		// Responses available to all API actions indexed by name
		Responses map[string]*ResponseDefinition
		// Response template factories available to all API actions indexed by name
		ResponseTemplates map[string]*ResponseTemplateDefinition
		// Built-in responses
		DefaultResponses map[string]*ResponseDefinition
		// Built-in response templates
		DefaultResponseTemplates map[string]*ResponseTemplateDefinition
		// DSLFunc contains the DSL used to create this definition if any.
		DSLFunc func()
		// Metadata is a list of key/value pairs
		Metadata dslengine.MetadataDefinition
	}

	// ContactDefinition contains the API contact information.
	ContactDefinition struct {
		// Name of the contact person/organization
		Name string `json:"name,omitempty"`
		// Email address of the contact person/organization
		Email string `json:"email,omitempty"`
		// URL pointing to the contact information
		URL string `json:"url,omitempty"`
	}

	// LicenseDefinition contains the license information for the API.
	LicenseDefinition struct {
		// Name of license used for the API
		Name string `json:"name,omitempty"`
		// URL to the license used for the API
		URL string `json:"url,omitempty"`
	}

	// DocsDefinition points to external documentation.
	DocsDefinition struct {
		// Description of documentation.
		Description string `json:"description,omitempty"`
		// URL to documentation.
		URL string `json:"url,omitempty"`
	}

	// ResourceDefinition describes a REST resource.
	// It defines both a media type and a set of actions that can be executed through HTTP
	// requests.
	// A resource is versioned so that multiple versions of the same resource may be exposed
	// by the API.
	ResourceDefinition struct {
		// Resource name
		Name string
		// Common URL prefix to all resource action HTTP requests
		BasePath string
		// Object describing each parameter that appears in BasePath if any
		BaseParams *AttributeDefinition
		// Name of parent resource if any
		ParentName string
		// Optional description
		Description string
		// API versions that expose this resource.
		APIVersions []string
		// Default media type, describes the resource attributes
		MediaType string
		// Exposed resource actions indexed by name
		Actions map[string]*ActionDefinition
		// Action with canonical resource path
		CanonicalActionName string
		// Map of response definitions that apply to all actions indexed by name.
		Responses map[string]*ResponseDefinition
		// Path and query string parameters that apply to all actions.
		Params *AttributeDefinition
		// Request headers that apply to all actions.
		Headers *AttributeDefinition
		// DSLFunc contains the DSL used to create this definition if any.
		DSLFunc func()
		// metadata is a list of key/value pairs
		Metadata dslengine.MetadataDefinition
	}

	// EncodingDefinition defines an encoder supported by the API.
	EncodingDefinition struct {
		// MIMETypes is the set of possible MIME types for the content being encoded or decoded.
		MIMETypes []string
		// PackagePath is the path to the Go package that implements the encoder / decoder.
		// The package must expose a `EncoderFactory` or `DecoderFactory` function
		// that the generated code calls. The methods must return objects that implement
		// the goa.EncoderFactory or goa.DecoderFactory interface respectively.
		PackagePath string
	}

	// ResponseDefinition defines a HTTP response status and optional validation rules.
	ResponseDefinition struct {
		// Response name
		Name string
		// HTTP status
		Status int
		// Response description
		Description string
		// Response body type if any
		Type DataType
		// Response body media type if any
		MediaType string
		// Response header definitions
		Headers *AttributeDefinition
		// Parent action or resource
		Parent dslengine.Definition
		// Metadata is a list of key/value pairs
		Metadata dslengine.MetadataDefinition
		// Standard is true if the response definition comes from the goa default responses
		Standard bool
		// Global is true if the response definition comes from the global API properties
		Global bool
	}

	// ResponseTemplateDefinition defines a response template.
	// A response template is a function that takes an arbitrary number
	// of strings and returns a response definition.
	ResponseTemplateDefinition struct {
		// Response template name
		Name string
		// Response template function
		Template func(params ...string) *ResponseDefinition
	}

	// ActionDefinition defines a resource action.
	// It defines both an HTTP endpoint and the shape of HTTP requests and responses made to
	// that endpoint.
	// The shape of requests is defined via "parameters", there are path parameters
	// parameters and a payload parameter (request body).
	// (i.e. portions of the URL that define parameter values), query string
	ActionDefinition struct {
		// Action name, e.g. "create"
		Name string
		// Action description, e.g. "Creates a task"
		Description string
		// Docs points to the API external documentation
		Docs *DocsDefinition
		// Parent resource
		Parent *ResourceDefinition
		// Specific action URL schemes
		Schemes []string
		// Action routes
		Routes []*RouteDefinition
		// Map of possible response definitions indexed by name
		Responses map[string]*ResponseDefinition
		// Path and query string parameters
		Params *AttributeDefinition
		// Query string parameters only
		QueryParams *AttributeDefinition
		// Payload blueprint (request body) if any
		Payload *UserTypeDefinition
		// Request headers that need to be made available to action
		Headers *AttributeDefinition
		// Metadata is a list of key/value pairs
		Metadata dslengine.MetadataDefinition
	}

	// LinkDefinition defines a media type link, it specifies a URL to a related resource.
	LinkDefinition struct {
		// Link name
		Name string
		// View used to render link if not "link"
		View string
		// URITemplate is the RFC6570 URI template of the link Href.
		URITemplate string

		// Parent media Type
		Parent *MediaTypeDefinition
	}

	// ViewDefinition defines which members and links to render when building a response.
	// The view is a JSON object whose property names must match the names of the parent media
	// type members.
	// The members fields are inherited from the parent media type but may be overridden.
	ViewDefinition struct {
		// Set of properties included in view
		*AttributeDefinition
		// Name of view
		Name string
		// Parent media Type
		Parent *MediaTypeDefinition
	}

	// RouteDefinition represents an action route.
	RouteDefinition struct {
		// Verb is the HTTP method, e.g. "GET", "POST", etc.
		Verb string
		// Path is the URL path e.g. "/tasks/:id"
		Path string
		// Parent is the action this route applies to.
		Parent *ActionDefinition
	}

	// ResourceIterator is the type of functions given to IterateResources.
	ResourceIterator func(r *ResourceDefinition) error

	// MediaTypeIterator is the type of functions given to IterateMediaTypes.
	MediaTypeIterator func(m *MediaTypeDefinition) error

	// UserTypeIterator is the type of functions given to IterateUserTypes.
	UserTypeIterator func(m *UserTypeDefinition) error

	// ActionIterator is the type of functions given to IterateActions.
	ActionIterator func(a *ActionDefinition) error

	// ResponseIterator is the type of functions given to IterateResponses.
	ResponseIterator func(r *ResponseDefinition) error

	// MediaTypeRoot is the data structure that represents the additional DSL definition root
	// that contains the media type definition set created by CollectionOf.
	MediaTypeRoot map[string]*MediaTypeDefinition
)

// Context returns the generic definition name used in error messages.
func (a *APIDefinition) Context() string {
	if a.Name != "" {
		return fmt.Sprintf("API %#v", a.Name)
	}
	return "unnamed API"
}

// IterateMediaTypes calls the given iterator passing in each media type sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateMediaTypes returns that
// error.
func (a *APIDefinition) IterateMediaTypes(it MediaTypeIterator) error {
	names := make([]string, len(a.MediaTypes))
	i := 0
	for n := range a.MediaTypes {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(a.MediaTypes[n]); err != nil {
			return err
		}
	}
	return nil
}

// IterateUserTypes calls the given iterator passing in each user type sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateUserTypes returns that
// error.
func (a *APIDefinition) IterateUserTypes(it UserTypeIterator) error {
	names := make([]string, len(a.Types))
	i := 0
	for n := range a.Types {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(a.Types[n]); err != nil {
			return err
		}
	}
	return nil
}

// GenerateExample returns a random value for the given data type.
// If the data type has validations then the example value validates them.
// GenerateExample returns the same random value for a given api name
// (the random generator is seeded after the api name).
func (a *APIDefinition) GenerateExample(dt DataType) interface{} {
	return dt.GenerateExample(a.RandomGenerator())
}

// RandomGenerator is seeded after the API name. It's used to generate examples.
func (a *APIDefinition) RandomGenerator() *RandomGenerator {
	if a.rand == nil {
		a.rand = NewRandomGenerator(a.Name)
	}
	return a.rand
}

// MediaTypeWithIdentifier returns the media type with a matching
// media type identifier. Two media type identifiers match if their
// values sans suffix match. So for example "application/vnd.foo+xml",
// "application/vnd.foo+json" and "application/vnd.foo" all match.
func (a *APIDefinition) MediaTypeWithIdentifier(id string) *MediaTypeDefinition {
	canonicalID := CanonicalIdentifier(id)
	var mtwi *MediaTypeDefinition
	for _, mt := range a.MediaTypes {
		if canonicalID == CanonicalIdentifier(mt.Identifier) {
			mtwi = mt
			break
		}
	}
	return mtwi
}

// IterateResources calls the given iterator passing in each resource sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateResources returns that
// error.
func (a *APIDefinition) IterateResources(it ResourceIterator) error {
	names := make([]string, len(a.Resources))
	i := 0
	for n := range a.Resources {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(a.Resources[n]); err != nil {
			return err
		}
	}
	return nil
}

// IterateVersions calls the given iterator passing in each API version definition sorted
// alphabetically by version name. It first calls the iterator on the embedded version definition
// which contains the definitions for all the unversioned resources.
// Iteration stops if an iterator returns an error and in this case IterateVersions returns that
// error.
func (a *APIDefinition) IterateVersions(it VersionIterator) error {
	versions := make([]string, len(a.APIVersions))
	i := 0
	for n := range a.APIVersions {
		versions[i] = n
		i++
	}
	sort.Strings(versions)
	if err := it(Design.APIVersionDefinition); err != nil {
		return err
	}
	for _, v := range versions {
		if err := it(Design.APIVersions[v]); err != nil {
			return err
		}
	}
	return nil
}

// Versions returns an array of supported versions.
func (a *APIDefinition) Versions() (versions []string) {
	a.IterateVersions(func(v *APIVersionDefinition) error {
		if v.Version != "" {
			versions = append(versions, v.Version)
		}
		return nil
	})
	return
}

// IterateSets goes over all the definition sets of the API: The API definition itself, each
// version definition, user types, media types and finally resources.
func (a *APIDefinition) IterateSets(iterator dslengine.SetIterator) {
	// First run the top level API DSL to initialize responses and
	// response templates needed by resources.
	iterator([]dslengine.Definition{a})

	// Then all the versions
	sortedVersions := make([]dslengine.Definition, len(a.APIVersions))
	i := 0
	a.IterateVersions(func(ver *APIVersionDefinition) error {
		if !ver.IsDefault() {
			sortedVersions[i] = ver
			i++
		}
		return nil
	})
	iterator(sortedVersions)

	// Then run the user type DSLs
	typeAttributes := make([]dslengine.Definition, len(a.Types))
	i = 0
	a.IterateUserTypes(func(u *UserTypeDefinition) error {
		u.AttributeDefinition.DSLFunc = u.DSLFunc
		typeAttributes[i] = u.AttributeDefinition
		i++
		return nil
	})
	iterator(typeAttributes)

	// Then the media type DSLs
	mediaTypes := make([]dslengine.Definition, len(a.MediaTypes))
	i = 0
	a.IterateMediaTypes(func(mt *MediaTypeDefinition) error {
		mediaTypes[i] = mt
		i++
		return nil
	})
	iterator(mediaTypes)

	// And now that we have everything the resources.
	resources := make([]dslengine.Definition, len(a.Resources))
	i = 0
	a.IterateResources(func(res *ResourceDefinition) error {
		resources[i] = res
		i++
		return nil
	})
	iterator(resources)
}

// SupportsVersion returns true if the object supports the given version.
func (a *APIDefinition) SupportsVersion(ver string) bool {
	found := fmt.Errorf("found")
	res := a.IterateVersions(func(v *APIVersionDefinition) error {
		if v.Version == ver {
			return found
		}
		return nil
	})
	return res == found
}

// SupportsNoVersion returns true if the API is unversioned.
func (a *APIDefinition) SupportsNoVersion() bool {
	return len(a.APIVersions) == 0
}

// Context returns the generic definition name used in error messages.
func (v *APIVersionDefinition) Context() string {
	if v.Version != "" {
		return fmt.Sprintf("%s version %s", Design.Context(), v.Version)
	}
	return Design.Context()
}

// Finalize sets the Consumes and Produces fields to the defaults if empty.
func (v *APIVersionDefinition) Finalize() {
	if len(v.Consumes) == 0 {
		v.Consumes = DefaultDecoders
	}
	if len(v.Produces) == 0 {
		v.Produces = DefaultEncoders
	}
}

// IsDefault returns true if the version definition applies to all versions (i.e. is the API
// definition).
func (v *APIVersionDefinition) IsDefault() bool {
	return v.Version == ""
}

// IterateResources calls the given iterator passing in each resource sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateResources returns that
// error.
func (v *APIVersionDefinition) IterateResources(it ResourceIterator) error {
	var names []string
	for n, res := range Design.Resources {
		if res.SupportsVersion(v.Version) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(Design.Resources[n]); err != nil {
			return err
		}
	}
	return nil
}

// IterateMediaTypes calls the given iterator passing in each media type sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateMediaTypes returns that
// error.
func (v *APIVersionDefinition) IterateMediaTypes(it MediaTypeIterator) error {
	var names []string
	for n, mt := range Design.MediaTypes {
		if mt.SupportsVersion(v.Version) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(Design.MediaTypes[n]); err != nil {
			return err
		}
	}
	return nil
}

// IterateUserTypes calls the given iterator passing in each user type sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateUserTypes returns that
// error.
func (v *APIVersionDefinition) IterateUserTypes(it UserTypeIterator) error {
	var names []string
	for n, ut := range Design.Types {
		if ut.SupportsVersion(v.Version) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(Design.Types[n]); err != nil {
			return err
		}
	}
	return nil
}

// IterateResponses calls the given iterator passing in each response sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateResponses returns that
// error.
func (v *APIVersionDefinition) IterateResponses(it ResponseIterator) error {
	names := make([]string, len(v.Responses))
	i := 0
	for n := range v.Responses {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(v.Responses[n]); err != nil {
			return err
		}
	}
	return nil
}

// DSL returns the initialization DSL.
func (v *APIVersionDefinition) DSL() func() {
	return v.DSLFunc
}

// CanonicalIdentifier returns the media type identifier sans suffix
// which is what the DSL uses to store and lookup media types.
func CanonicalIdentifier(identifier string) string {
	base, params, err := mime.ParseMediaType(identifier)
	if err != nil {
		return identifier
	}
	id := base
	if i := strings.Index(id, "+"); i != -1 {
		id = id[:i]
	}
	return mime.FormatMediaType(id, params)
}

// NewResourceDefinition creates a resource definition but does not
// execute the DSL.
func NewResourceDefinition(name string, dsl func()) *ResourceDefinition {
	return &ResourceDefinition{
		Name:      name,
		MediaType: "plain/text",
		DSLFunc:   dsl,
	}
}

// Context returns the generic definition name used in error messages.
func (r *ResourceDefinition) Context() string {
	if r.Name != "" {
		return fmt.Sprintf("resource %#v", r.Name)
	}
	return "unnamed resource"
}

// IterateActions calls the given iterator passing in each resource action sorted in alphabetical order.
// Iteration stops if an iterator returns an error and in this case IterateActions returns that
// error.
func (r *ResourceDefinition) IterateActions(it ActionIterator) error {
	names := make([]string, len(r.Actions))
	i := 0
	for n := range r.Actions {
		names[i] = n
		i++
	}
	sort.Strings(names)
	for _, n := range names {
		if err := it(r.Actions[n]); err != nil {
			return err
		}
	}
	return nil
}

// CanonicalAction returns the canonical action of the resource if any.
// The canonical action is used to compute hrefs to resources.
func (r *ResourceDefinition) CanonicalAction() *ActionDefinition {
	name := r.CanonicalActionName
	if name == "" {
		name = "show"
	}
	ca, _ := r.Actions[name]
	return ca
}

// URITemplate returns a httprouter compliant URI template to this resource.
// The result is the empty string if the resource does not have a "show" action
// and does not define a different canonical action.
func (r *ResourceDefinition) URITemplate(version *APIVersionDefinition) string {
	ca := r.CanonicalAction()
	if ca == nil || len(ca.Routes) == 0 {
		return ""
	}
	return ca.Routes[0].FullPath(version)
}

// FullPath computes the base path to the resource actions concatenating the API and parent resource
// base paths as needed.
func (r *ResourceDefinition) FullPath(version *APIVersionDefinition) string {
	var basePath string
	if p := r.Parent(); p != nil {
		if ca := p.CanonicalAction(); ca != nil {
			if routes := ca.Routes; len(routes) > 0 {
				// Note: all these tests should be true at code generation time
				// as DSL validation makes sure that parent resources have a
				// canonical path.
				basePath = path.Join(routes[0].FullPath(version))
			}
		}
	} else {
		basePath = version.BasePath
	}
	return httprouter.CleanPath(path.Join(basePath, r.BasePath))
}

// Parent returns the parent resource if any, nil otherwise.
func (r *ResourceDefinition) Parent() *ResourceDefinition {
	if r.ParentName != "" {
		if parent, ok := Design.Resources[r.ParentName]; ok {
			return parent
		}
	}
	return nil
}

// Versions returns the API versions that expose the resource.
func (r *ResourceDefinition) Versions() []string {
	return r.APIVersions
}

// SupportsVersion returns true if the resource is exposed by the given API version.
// An empty string version means no version.
func (r *ResourceDefinition) SupportsVersion(version string) bool {
	if version == "" {
		return r.SupportsNoVersion()
	}
	for _, v := range r.APIVersions {
		if v == version {
			return true
		}
	}
	return false
}

// SupportsNoVersion returns true if the resource is exposed by an unversioned API.
func (r *ResourceDefinition) SupportsNoVersion() bool {
	return len(r.APIVersions) == 0
}

// DSL returns the initialization DSL.
func (r *ResourceDefinition) DSL() func() {
	return r.DSLFunc
}

// Finalize is run post DSL execution. It merges response definitions, creates implicit action
// parameters, initializes querystring parameters and sets path parameters as non zero attributes.
func (r *ResourceDefinition) Finalize() {
	r.IterateActions(func(a *ActionDefinition) error {
		// 1. Merge response definitions
		for name, resp := range a.Responses {
			if pr, ok := a.Parent.Responses[name]; ok {
				resp.Merge(pr)
			}
			if ar, ok := Design.Responses[name]; ok {
				resp.Merge(ar)
			}
			if dr, ok := Design.DefaultResponses[name]; ok {
				resp.Merge(dr)
			}
		}
		// 2. Create implicit action parameters for path wildcards that dont' have one
		for _, r := range a.Routes {
			Design.IterateVersions(func(ver *APIVersionDefinition) error {
				wcs := ExtractWildcards(r.FullPath(ver))
				for _, wc := range wcs {
					found := false
					var o Object
					if all := a.Params; all != nil {
						o = all.Type.ToObject()
					} else {
						o = Object{}
						a.Params = &AttributeDefinition{Type: o}
					}
					for n := range o {
						if n == wc {
							found = true
							break
						}
					}
					if !found {
						o[wc] = &AttributeDefinition{Type: String}
					}
				}
				return nil
			})
		}
		// 3. Compute QueryParams from Params and set all path params as non zero attributes
		if params := a.Params; params != nil {
			queryParams := DupAtt(params)
			a.Params.NonZeroAttributes = make(map[string]bool)
			Design.IterateVersions(func(ver *APIVersionDefinition) error {
				for _, route := range a.Routes {
					pnames := route.Params(ver)
					for _, pname := range pnames {
						a.Params.NonZeroAttributes[pname] = true
						delete(queryParams.Type.ToObject(), pname)
					}
				}
				return nil
			})
			// (note: we may end up with required attribute names that don't correspond
			// to actual attributes cos' we just deleted them but that's probably OK.)
			a.QueryParams = queryParams
		}

		return nil
	})
}

// Context returns the generic definition name used in error messages.
func (enc *EncodingDefinition) Context() string {
	return fmt.Sprintf("encoding for %s", strings.Join(enc.MIMETypes, ", "))
}

// HasKnownEncoder returns true if the encoder for the given MIME type is known by goa.
// MIME types with unknown encoders must be associated with a package path explicitly in the DSL.
func HasKnownEncoder(mimeType string) bool {
	return KnownEncoders[mimeType][1] != ""
}

// IsGoaEncoder returns true if the encoder for the given MIME type is implemented in the goa
// package.
func IsGoaEncoder(pkgPath string) bool {
	return pkgPath == "json" || pkgPath == "xml" || pkgPath == "gob"
}

// SupportingPackages returns the package paths to the packages that implements the encoders and
// decoders that support the MIME types in the definition.
// The return value maps the package path to the corresponding list of supported MIME types.
// It is nil if no package could be found for *any* of the MIME types in the definition (in which
// case the definition is invalid).
func (enc *EncodingDefinition) SupportingPackages() map[string][]string {
	if enc.PackagePath != "" {
		return map[string][]string{enc.PackagePath: enc.MIMETypes}
	}
	mimeTypes := enc.MIMETypes
	if len(mimeTypes) == 0 || !HasKnownEncoder(mimeTypes[0]) {
		// invalid definition
		return nil
	}
	ppath := KnownEncoders[mimeTypes[0]][0]
	paths := map[string][]string{ppath: {mimeTypes[0]}}
	if len(mimeTypes) == 1 {
		return paths
	}
	for _, m := range mimeTypes[1:] {
		if !HasKnownEncoder(m) {
			return nil
		}
		e := KnownEncoders[m][0]
		if existing, ok := paths[e]; ok {
			paths[e] = append(existing, m)
		} else {
			paths[e] = []string{m}
		}
	}
	return paths
}

// Context returns the generic definition name used in error messages.
func (c *ContactDefinition) Context() string {
	if c.Name != "" {
		return fmt.Sprintf("contact %s", c.Name)
	}
	return "unnamed contact"
}

// Context returns the generic definition name used in error messages.
func (l *LicenseDefinition) Context() string {
	if l.Name != "" {
		return fmt.Sprintf("license %s", l.Name)
	}
	return "unnamed license"
}

// Context returns the generic definition name used in error messages.
func (d *DocsDefinition) Context() string {
	return fmt.Sprintf("documentation for %s", Design.Name)
}

// Context returns the generic definition name used in error messages.
func (t *UserTypeDefinition) Context() string {
	if t.TypeName != "" {
		return fmt.Sprintf("type %#v", t.TypeName)
	}
	return "unnamed type"
}

// DSL returns the initialization DSL.
func (t *UserTypeDefinition) DSL() func() {
	return t.DSLFunc
}

// Context returns the generic definition name used in error messages.
func (r *ResponseDefinition) Context() string {
	var prefix, suffix string
	if r.Name != "" {
		prefix = fmt.Sprintf("response %#v", r.Name)
	} else {
		prefix = "unnamed response"
	}
	if r.Parent != nil {
		suffix = fmt.Sprintf(" of %s", r.Parent.Context())
	}
	return prefix + suffix
}

// Dup returns a copy of the response definition.
func (r *ResponseDefinition) Dup() *ResponseDefinition {
	res := ResponseDefinition{
		Name:        r.Name,
		Status:      r.Status,
		Description: r.Description,
		MediaType:   r.MediaType,
	}
	if r.Headers != nil {
		res.Headers = DupAtt(r.Headers)
	}
	return &res
}

// Merge merges other into target. Only the fields of target that are not already set are merged.
func (r *ResponseDefinition) Merge(other *ResponseDefinition) {
	if other == nil {
		return
	}
	if r.Name == "" {
		r.Name = other.Name
	}
	if r.Status == 0 {
		r.Status = other.Status
	}
	if r.Description == "" {
		r.Description = other.Description
	}
	if r.MediaType == "" {
		r.MediaType = other.MediaType
	}
	if other.Headers != nil {
		otherHeaders := other.Headers.Type.ToObject()
		if len(otherHeaders) > 0 {
			if r.Headers == nil {
				r.Headers = &AttributeDefinition{Type: Object{}}
			}
			headers := r.Headers.Type.ToObject()
			for n, h := range otherHeaders {
				if _, ok := headers[n]; !ok {
					headers[n] = h
				}
			}
		}
	}
}

// Context returns the generic definition name used in error messages.
func (r *ResponseTemplateDefinition) Context() string {
	if r.Name != "" {
		return fmt.Sprintf("response template %#v", r.Name)
	}
	return "unnamed response template"
}

// Context returns the generic definition name used in error messages.
func (a *ActionDefinition) Context() string {
	var prefix, suffix string
	if a.Name != "" {
		suffix = fmt.Sprintf(" action %#v", a.Name)
	} else {
		suffix = " unnamed action"
	}
	if a.Parent != nil {
		prefix = a.Parent.Context()
	}
	return prefix + suffix
}

// PathParams returns the path parameters of the action across all its routes.
func (a *ActionDefinition) PathParams(version *APIVersionDefinition) *AttributeDefinition {
	obj := make(Object)
	for _, r := range a.Routes {
		for _, p := range r.Params(version) {
			if _, ok := obj[p]; !ok {
				obj[p] = a.Params.Type.ToObject()[p]
			}
		}
	}
	res := &AttributeDefinition{Type: obj}
	if a.HasAbsoluteRoutes() {
		return res
	}
	res = res.Merge(a.Parent.BaseParams)
	res = res.Merge(Design.BaseParams)
	if p := a.Parent.Parent(); p != nil {
		res = res.Merge(p.CanonicalAction().PathParams(version))
	}
	return res
}

// AllParams returns the path and query string parameters of the action across all its routes.
func (a *ActionDefinition) AllParams() *AttributeDefinition {
	var res *AttributeDefinition
	if a.Params != nil {
		res = DupAtt(a.Params)
	} else {
		res = &AttributeDefinition{Type: Object{}}
	}
	if a.HasAbsoluteRoutes() {
		return res
	}
	res = res.Merge(a.Parent.BaseParams)
	res = res.Merge(Design.BaseParams)
	if p := a.Parent.Parent(); p != nil {
		res = res.Merge(p.CanonicalAction().AllParams())
	}
	return res
}

// HasAbsoluteRoutes returns true if all the action routes are absolute.
func (a *ActionDefinition) HasAbsoluteRoutes() bool {
	for _, r := range a.Routes {
		if !r.IsAbsolute() {
			return false
		}
	}
	return true
}

// Context returns the generic definition name used in error messages.
func (l *LinkDefinition) Context() string {
	var prefix, suffix string
	if l.Name != "" {
		prefix = fmt.Sprintf("link %#v", l.Name)
	} else {
		prefix = "unnamed link"
	}
	if l.Parent != nil {
		suffix = fmt.Sprintf(" of %s", l.Parent.Context())
	}
	return prefix + suffix
}

// Attribute returns the linked attribute.
func (l *LinkDefinition) Attribute() *AttributeDefinition {
	p := l.Parent.ToObject()
	if p == nil {
		return nil
	}
	att, _ := p[l.Name]

	return att
}

// MediaType returns the media type of the linked attribute.
func (l *LinkDefinition) MediaType() *MediaTypeDefinition {
	att := l.Attribute()
	mt, _ := att.Type.(*MediaTypeDefinition)
	return mt
}

// Context returns the generic definition name used in error messages.
func (v *ViewDefinition) Context() string {
	var prefix, suffix string
	if v.Name != "" {
		prefix = fmt.Sprintf("view %#v", v.Name)
	} else {
		prefix = "unnamed view"
	}
	if v.Parent != nil {
		suffix = fmt.Sprintf(" of %s", v.Parent.Context())
	}
	return prefix + suffix
}

// Context returns the generic definition name used in error messages.
func (r *RouteDefinition) Context() string {
	return fmt.Sprintf(`route %s "%s" of %s`, r.Verb, r.Path, r.Parent.Context())
}

// Params returns the route parameters.
// For example for the route "GET /foo/:fooID" Params returns []string{"fooID"}.
func (r *RouteDefinition) Params(version *APIVersionDefinition) []string {
	return ExtractWildcards(r.FullPath(version))
}

// FullPath returns the action full path computed by concatenating the API and resource base paths
// with the action specific path.
func (r *RouteDefinition) FullPath(version *APIVersionDefinition) string {
	if r.IsAbsolute() {
		return httprouter.CleanPath(r.Path[1:])
	}
	var base string
	if r.Parent != nil && r.Parent.Parent != nil {
		base = r.Parent.Parent.FullPath(version)
	}
	return httprouter.CleanPath(path.Join(base, r.Path))
}

// IsAbsolute returns true if the action path should not be concatenated to the resource and API
// base paths.
func (r *RouteDefinition) IsAbsolute() bool {
	return strings.HasPrefix(r.Path, "//")
}

// IterateSets iterates over the one generated media type definition set.
func (r MediaTypeRoot) IterateSets(iterator dslengine.SetIterator) {
	canonicalIDs := make([]string, len(r))
	i := 0
	for _, mt := range r {
		canonicalID := CanonicalIdentifier(mt.Identifier)
		Design.MediaTypes[canonicalID] = mt
		canonicalIDs[i] = canonicalID
		i++
	}
	sort.Strings(canonicalIDs)
	set := make([]dslengine.Definition, len(canonicalIDs))
	for i, cid := range canonicalIDs {
		set[i] = Design.MediaTypes[cid]
	}
	iterator(set)
}

// ExtractWildcards returns the names of the wildcards that appear in path.
func ExtractWildcards(path string) []string {
	matches := WildcardRegex.FindAllStringSubmatch(path, -1)
	wcs := make([]string, len(matches))
	for i, m := range matches {
		wcs[i] = m[1]
	}
	return wcs
}
