// Package asset helps using static assets (scripts, stylesheets, other files) prepared by external
// asset pipelines in Go (Golang) templates. It provides template functions that insert references to
// (optionally) minified and versioned files.
//
// The idea behind this package is that in some cases creating asset bundles is best left to external
// tools such as grunt or gulp. The default configuration relies on a presence of a JSON file describing
// a mapping from original to minified assets. Such file can be prepared e.g. by gulp-rev.
//
// Example of such file:
//     {
//       "js/main.min.js": "js/main.min-da89a0c4.js",
//       "css/style.min.css": "css/style.min-16680603.css"
//     }
//
// Example usage in template:
//
//     <head>
//         {{ linktag "css/style.css" }}
//         {{ scripttag "js/main.js" }}
//
//         <!-- Additional attributes can be passed using an even number of arguments: -->
//         {{ scripttag "js/main.js" "charset" "UTF-8" }}
//     </head>
//     <body>
//         <!-- Inserts URL prefix to avoid hardcoding it -->
//         <img src="{{ static }}/img/logo.jpg"/>
//     </body>
//
// Example initialization:
//     import (
//         "github.com/rsniezynski/go-asset-helper"
//         "html/template"
//     )
//
//     func main() {
//         static, err := asset.NewStatic("/static/", "/path/to/manifest.json")
//         if err != nil {
//             // Manifest file doesn't exist or is not a valid JSON
//         }
//         tmpl := template.Must(template.ParseFiles("template_name.html"))
//
//         // Attach helper functions with the template:
//         static.Attach(tmpl)
//         // Alternatively:
//         tmpl.Funcs(static.FuncMap())
//
//         // Use minified versions if available:
//         static, err = asset.NewStatic("/static/", "/path/to/manifest.json", asset.WithUseMinified(true))
//
//         // Use minified versions and load the manifest file using go-bindata (Asset is a go-bindata class).
//         // The loader is a func(string) ([]byte, error)
//         static, err = asset.NewStatic(
//             "/static/", "/path/to/manifest.json",
//             asset.WithUseMinified(true),
//             asset.WithManifestLoader(Asset),
//         )
//
//         // There's also WithMappingBuilder option to create an asset mapper without
//         // using the manifest file.
//     }
package asset

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

// Static holds configurtion for the asset resolver. It should be created using NewStatic
type Static struct {
	urlPrefix      string
	manifestPath   string
	manifestLoader Loader
	useMinified    bool
	mapping        StaticMapper
	mappingBuilder MappingBuilder
}

// NewStatic creates an instance of Static, which can be then used to attach helper functions to templates.
func NewStatic(urlPrefix string, manifestPath string, options ...optionSetter) (*Static, error) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	static := &Static{
		urlPrefix:      urlPrefix,
		manifestPath:   manifestPath,
		manifestLoader: ioutil.ReadFile,
	}
	for _, optionSetter := range options {
		optionSetter(static)
	}
	if static.mappingBuilder == nil {
		static.mappingBuilder = func() (StaticMapper, error) {
			return createMapping(static.manifestLoader, manifestPath, static.useMinified)
		}
	}
	mapping, err := static.mappingBuilder()
	if err != nil {
		return nil, err
	}
	static.mapping = mapping
	return static, nil
}

// ScriptTag returns HTML script tag; path should point to an asset, by default a path on the disk
// relative to the directory from which the application process is started. This behavior can
// be modified by providing a different loader on Static object creation.
// attrs can be used to pass additional attributes to the tag. There must be an even numner of
// attrs.
func (st *Static) ScriptTag(path string, attrs ...string) (template.HTML, error) {
	defaultAttrMap := map[string]string{"type": "text/javascript"}
	attrMap, err := attrSliceToMap(attrs)
	if err != nil {
		return "", err
	}
	updateMap(defaultAttrMap, attrMap)
	defaultAttrMap["src"] = st.urlPrefix + st.mapping.Get(path)
	return template.HTML(fmt.Sprintf(`<script %s></script>`, mapToAttrs(defaultAttrMap))), nil
}

// LinkTag returns HTML script tag. See ScriptTag for additional information.
func (st *Static) LinkTag(path string, attrs ...string) (template.HTML, error) {
	defaultAttrMap := map[string]string{"type": "text/css", "rel": "stylesheet"}
	attrMap, err := attrSliceToMap(attrs)
	if err != nil {
		return "", err
	}
	updateMap(defaultAttrMap, attrMap)
	defaultAttrMap["href"] = st.urlPrefix + st.mapping.Get(path)
	return template.HTML(fmt.Sprintf(`<link %s/>`, mapToAttrs(defaultAttrMap))), nil
}

// Static returns URL prefix for static assets. Mainly intended to be used for image files etc.
func (st *Static) Static() template.HTML {
	return template.HTML(st.urlPrefix)
}

// Attach sets ScriptTag, LinkTag and Stastic as, respectively, scripttag, linktag and static template functions.
func (st *Static) Attach(tmpl *template.Template) {
	tmpl.Funcs(st.FuncMap())
}

// FuncMap returns template.FuncMap that can be used to attach go-asset-helper functions
// to a template.
func (st *Static) FuncMap() template.FuncMap {
	return map[string]interface{}{
		"scripttag": st.ScriptTag,
		"linktag":   st.LinkTag,
		"static":    st.Static,
	}
}

// StaticMapper is an interface for mapping between asset paths and references to be put
// in template tags
type StaticMapper interface {
	// Get returns reference to an specified as a path.
	Get(string) string
}

// MappingBuilder is a function that produces StaticMapper instances
type MappingBuilder func() (StaticMapper, error)

type StaticMap struct {
	innerMap    map[string]interface{}
	useMinified bool
}

func (sm StaticMap) Get(name string) string {
	if sm.useMinified {
		minifiedName := toMinifiedName(name)
		if value, ok := getStringFromMap(sm.innerMap, minifiedName); ok {
			return value
		}
	}
	value, _ := getStringFromMap(sm.innerMap, name)
	return value
}

func getStringFromMap(amap map[string]interface{}, defaultValue string) (string, bool) {
	value, ok := amap[defaultValue]
	if ok {
		switch val := value.(type) {
		case string:
			return val, true
		default:
			return defaultValue, false
		}
	}
	return defaultValue, false
}

func toMinifiedName(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext) + ".min" + ext
}

func createMapping(load Loader, path string, useMinified bool) (StaticMapper, error) {
	var manifest interface{}
	if load != nil {
		content, err := load(path)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(content, &manifest)
		if err != nil {
			return nil, err
		}
		return &StaticMap{manifest.(map[string]interface{}), useMinified}, nil
	}
	return &StaticMap{map[string]interface{}{}, useMinified}, nil
}

type optionSetter func(*Static)

// Loader returns file contents for a given path
type Loader func(string) ([]byte, error)

// WithManifestLoader can be used to provide Loader implementation in NewStatic
func WithManifestLoader(load Loader) optionSetter {
	return func(st *Static) { st.manifestLoader = load }
}

// WithMappingBuilder can be used to provide MappingBuilder implementation in NewStatic
func WithMappingBuilder(builder MappingBuilder) optionSetter {
	return func(st *Static) { st.mappingBuilder = builder }
}

// WithUseMinified can be used in NewStatic to specify whether resoruce mapping should be used.
// false can be useful in debug mode.
func WithUseMinified(minified bool) optionSetter {
	return func(st *Static) { st.useMinified = minified }
}

func attrSliceToMap(attrsSlice []string) (map[string]string, error) {
	length := len(attrsSlice)
	if length%2 != 0 {
		return nil, errors.New("ScriptTag attributes don't form pairs")
	}
	attrMap := map[string]string{}
	for i := 0; i < length; i += 2 {
		attrMap[attrsSlice[i]] = attrsSlice[i+1]
	}
	return attrMap, nil
}

func mapToAttrs(attrMap map[string]string) string {
	attrSlice := make([]string, 0, len(attrMap))
	for key, value := range attrMap {
		attr := fmt.Sprintf(
			`%s="%s"`, html.EscapeString(key), html.EscapeString(value),
		)
		attrSlice = append(attrSlice, attr)
	}
	sort.StringSlice(attrSlice).Sort()
	return strings.Join(attrSlice, " ")
}

func updateMap(updated map[string]string, updating map[string]string) {
	for key, value := range updating {
		updated[key] = value
	}
}
