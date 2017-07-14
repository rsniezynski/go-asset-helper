package asset

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapToAttrs(t *testing.T) {
	params := map[string]string{
		"name":    "value",
		`"escape`: "me<",
		"other":   "attribute ",
	}
	require.Equal(t, `&#34;escape="me&lt;" name="value" other="attribute "`, mapToAttrs(params))
}

func TestAttrsSliceToMap(t *testing.T) {
	attrSlice := []string{"one", "two", "three", "four"}
	expected := map[string]string{"one": "two", "three": "four"}
	amap, err := attrSliceToMap(attrSlice)
	require.Nil(t, err)
	require.Equal(t, expected, amap)
}

func TestOddAttrsSliceToMap(t *testing.T) {
	attrSlice := []string{"one", "two", "three", "four", "five"}
	_, err := attrSliceToMap(attrSlice)
	require.NotNil(t, err)
}

func TestScriptTag(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"js/name.js":"dist/name-1234.js", "js/other.min.js":"dist/other-1234.min.js", "js/other.js": "dist/other-1234.js"}`,
		), nil
	}
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	tag, err := static.ScriptTag("js/other.js")
	require.Nil(t, err)
	require.Equal(t,
		`<script src="/static/dist/other-1234.min.js" type="text/javascript"></script>`, string(tag),
	)
}

func TestScriptTagParams(t *testing.T) {
	loader := func(name string) ([]byte, error) { return []byte(`{}`), nil }
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	tag, err := static.ScriptTag("js/other.js", "data-main", "some value", "defer", "defer")
	require.Nil(t, err)
	require.Equal(t,
		`<script data-main="some value" defer="defer" src="/static/js/other.js" type="text/javascript"></script>`, string(tag),
	)
}

func TestScriptTagOddParams(t *testing.T) {
	loader := func(name string) ([]byte, error) { return []byte(`{}`), nil }
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	_, err = static.ScriptTag("js/other.js", "data-main", "some value", "defer")
	require.NotNil(t, err)
}

func TestScriptTagSri(t *testing.T) {
	content := []byte("the quick brown fox jumps over the lazy dog\n")
	sha := "sha256-EVOkCA8fywRCWqC4QcKxRgb+bfJdkHbSofrOLVr1cSk="

	tmpfile, err := ioutil.TempFile("", "script-sri")
	require.Nil(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.Write(content)
	require.Nil(t, err)
	err = tmpfile.Close()
	require.Nil(t, err)

	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"script-sri":"` + strings.TrimPrefix(tmpfile.Name(), "/") + `"}`,
		), nil
	}

	static, err := NewStatic("", "data/manifest.json", WithManifestLoader(loader), WithUseSri(true))
	require.Nil(t, err)
	tag, err := static.ScriptTag("script-sri")
	require.Nil(t, err)

	expected := fmt.Sprintf(`<script crossorigin="anonymous" integrity="%s" src="%s" type="text/javascript"></script>`, sha, tmpfile.Name())
	require.Equal(t, expected, string(tag))
}

func TestLinkTag(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"js/name.js":"dist/name-1234.js", "css/other.min.css":"dist/other-1234.min.css", "js/other.js": "dist/other-1234.js"}`,
		), nil
	}
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	tag, err := static.LinkTag("css/other.css")
	require.Nil(t, err)
	require.Equal(t,
		`<link href="/static/dist/other-1234.min.css" rel="stylesheet" type="text/css"/>`, string(tag),
	)
}

func TestLinkTagParams(t *testing.T) {
	loader := func(name string) ([]byte, error) { return []byte(`{}`), nil }
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	tag, err := static.LinkTag("css/other.css", "media", "some value", "title", "whatever")
	require.Nil(t, err)
	require.Equal(t,
		`<link href="/static/css/other.css" media="some value" rel="stylesheet" title="whatever" type="text/css"/>`, string(tag),
	)
}

func TestLinkTagOddParams(t *testing.T) {
	loader := func(name string) ([]byte, error) { return []byte(`{}`), nil }
	static, err := NewStatic("/static", "data/manifest.json", WithManifestLoader(loader), WithUseMinified(true))
	require.Nil(t, err)
	_, err = static.LinkTag("css/other.css", "media", "some value", "title", "whatever", "odd")
	require.NotNil(t, err)
}

func TestLinkTagSri(t *testing.T) {
	content := []byte("the quick brown fox jumps over the lazy dog\n")
	sha := "sha256-EVOkCA8fywRCWqC4QcKxRgb+bfJdkHbSofrOLVr1cSk="

	tmpfile, err := ioutil.TempFile("", "link-sri")
	require.Nil(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.Write(content)
	require.Nil(t, err)
	err = tmpfile.Close()
	require.Nil(t, err)

	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"link-sri":"` + strings.TrimPrefix(tmpfile.Name(), "/") + `"}`,
		), nil
	}

	static, err := NewStatic("", "data/manifest.json", WithManifestLoader(loader), WithUseSri(true))
	require.Nil(t, err)
	tag, err := static.LinkTag("link-sri")
	require.Nil(t, err)

	expected := fmt.Sprintf(`<link crossorigin="anonymous" href="%s" integrity="%s" rel="stylesheet" type="text/css"/>`, tmpfile.Name(), sha)
	require.Equal(t, expected, string(tag))
}

func TestCreateMappingNoLoader(t *testing.T) {
	mapping, err := createMapping(nil, "filename", false)
	require.Nil(t, err)
	require.Equal(t, "name", mapping.Get("name"))
}

func TestCreateMappingFileError(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return nil, errors.New("I/O Error")
	}
	_, err := createMapping(loader, "filename", false)
	require.NotNil(t, err)
}

func TestCreateMappingInvalidJSON(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return []byte("garbage"), nil
	}
	_, err := createMapping(loader, "filename", false)
	require.NotNil(t, err)
}

func TestCreateMappingNotMinified(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"js/name.js":"dist/name-1234.js", "js/other.min.js":"dist/name-1234.min.js", "js/name.min.js":"dist/name-1234.min.js"}`,
		), nil
	}
	mapping, err := createMapping(loader, "filename", false)
	require.Nil(t, err)
	require.Equal(t, "dist/name-1234.js", mapping.Get("js/name.js"))
	require.Equal(t, "js/other.js", mapping.Get("js/other.js"))
}

func TestCreateMappingMinified(t *testing.T) {
	loader := func(name string) ([]byte, error) {
		return []byte(
			`{"js/name.js":"dist/name-1234.js", "js/other.min.js":"dist/other-1234.min.js", "js/other.js": "dist/other-1234.js"}`,
		), nil
	}
	mapping, err := createMapping(loader, "filename", true)
	require.Nil(t, err)
	require.Equal(t, "dist/name-1234.js", mapping.Get("js/name.js"))
	require.Equal(t, "dist/other-1234.min.js", mapping.Get("js/other.js"))
}
