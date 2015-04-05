# go-asset-helper

Package asset helps using static assets (scripts, stylesheets, other files)
prepared by external asset pipelines in Go (Golang) templates. It provides
template functions that insert references to (optionally) minified and versioned
files.

The idea behind this package is that in some cases creating asset bundles is
best left to external tools such as grunt or gulp. The default configuration
relies on a presence of a JSON file describing a mapping from original to
minified assets. Such file can be prepared e.g. by gulp-rev.

Example of a manifest file:
```
{
    "js/main.min.js": "js/main.min-da89a0c4.js",
    "css/style.min.css": "css/style.min-16680603.css"
}
```

Example usage in template:
```
<head>
    {{ linktag "css/style.css" }}
    {{ scripttag "js/main.js" }}

    <!-- Additional attributes can be passed using an even number of arguments: -->
    {{ scripttag "js/main.js" "charset" "UTF-8" }}
</head>
<body>
    <!-- Inserts URL prefix to avoid hardcoding it -->
    <img src="{{ static }}/img/logo.jpg"/>
</body>
```

Example initialization:
```
import (
    "github.com/rsniezynski/go-asset-helper"
    "html/template"
)

func main() {
    static, err := asset.NewStatic("/static/", "/path/to/manifest.json")
    if err != nil {
        // Manifest file doesn't exist or is not a valid JSON
    }
    tmpl := template.Must(template.ParseFiles("template_name.html"))

    // Attach helper functions with the template:
    static.Attach(tmpl)
    // Alternatively:
    tmpl.Funcs(static.FuncMap())

    // Use minified versions if available:
    static, err = asset.NewStatic("/static/", "/path/to/manifest.json", asset.WithUseMinified(true))

    // Use minified versions and load the manifest file using go-bindata (Asset is a go-bindata class).
    // The loader is a func(string) ([]byte, error)
    static, err = asset.NewStatic(
        "/static/", "/path/to/manifest.json",
        asset.WithUseMinified(true),
        asset.WithManifestLoader(Asset),
    )

    // There's also WithMappingBuilder option to create the asset mapper without
    // using the manifest file. No example at this time.
}
```
