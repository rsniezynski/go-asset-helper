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
//         // There's also WithMappingBuilder option to create the asset mapper without
//         // using the manifest file. No example at this time.
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

type Static struct {
	urlPrefix      string
	manifestPath   string
	manifestLoader Loader
	useMinified    bool
	mapping        StaticMapper
	mappingBuilder MappingBuilder
}

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

func (st *Static) Static() (template.HTML, error) {
	return template.HTML(st.urlPrefix), nil
}

func (st *Static) Attach(tmpl *template.Template) {
	tmpl.Funcs(st.FuncMap())
}

func (st *Static) FuncMap() template.FuncMap {
	return map[string]interface{}{
		"scripttag": st.ScriptTag,
		"linktag":   st.LinkTag,
		"static":    st.Static,
	}
}

type StaticMapper interface {
	Get(string) string
}

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

type Loader func(string) ([]byte, error)

func WithManifestLoader(load Loader) optionSetter {
	return func(st *Static) { st.manifestLoader = load }
}

func WithMappingBuilder(builder MappingBuilder) optionSetter {
	return func(st *Static) { st.mappingBuilder = builder }
}

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
