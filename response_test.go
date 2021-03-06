package air

import (
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestResponseHTTPResponseWriter(t *testing.T) {
	a := New()

	req, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)
	assert.Equal(t, a, res.Air)
	assert.Equal(t, req, res.req)
	assert.NotNil(t, res.hrw)

	hrw := res.HTTPResponseWriter()
	assert.NotNil(t, hrw)
	assert.Equal(t, res.Header, hrw.Header())
}

func TestResponseSetHTTPResponseWriter(t *testing.T) {
	a := New()

	_, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)

	hrw := httptest.NewRecorder()

	res.SetHTTPResponseWriter(hrw)
	assert.Equal(t, hrw, res.hrw)
	assert.Equal(t, hrw.Header(), res.Header)
	assert.Equal(t, hrw, res.Body)
}

func TestResponseSetCookie(t *testing.T) {
	a := New()

	_, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)

	res.SetCookie(&http.Cookie{})
	assert.Empty(t, res.Header.Get("Set-Cookie"))

	res.SetCookie(&http.Cookie{
		Name:  "foo",
		Value: "bar",
	})
	assert.Equal(t, "foo=bar", res.Header.Get("Set-Cookie"))
}

func TestResponseWrite(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.Write(nil))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Len(t, hrwrb, 0)

	_, res, hrw = fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.Write(strings.NewReader("foobar")))

	hrwr = hrw.Result()
	hrwrb, _ = ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(t, "foobar", string(hrwrb))

	_, res, hrw = fakeRRCycle(a, http.MethodHead, "/", nil)

	assert.NoError(t, res.Write(nil))
	assert.NoError(t, res.Write(strings.NewReader("foobar")))

	hrwr = hrw.Result()
	hrwrb, _ = ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Len(t, hrwrb, 0)

	_, res, _ = fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.Error(t, res.Write(&readErrorReader{
		Seeker: strings.NewReader("foobar"),
	}))
	assert.Error(t, res.Write(&seekErrorSeeker{
		Reader: strings.NewReader("foobar"),
	}))
	assert.NoError(t, res.Write(strings.NewReader("foobar")))
	assert.Equal(
		t,
		"text/plain; charset=utf-8",
		res.Header.Get("Content-Type"),
	)

	_, res, _ = fakeRRCycle(a, http.MethodGet, "/", nil)

	a.MinifierEnabled = true

	res.Header.Set("Content-Type", "text/html; charset=utf-8")
	assert.Error(t, res.Write(&readErrorReader{
		Seeker: strings.NewReader("<!DOCTYPE html>"),
	}))

	res.Header.Set("Content-Type", "application/json; charset=utf-8")
	assert.Error(t, res.Write(strings.NewReader("{")))

	res.Header.Set("Content-Type", "text/html; charset=utf-8")
	res.SetHTTPResponseWriter(&nopResponseWriter{
		ResponseWriter: res.HTTPResponseWriter(),
	})
	assert.NoError(t, res.Write(strings.NewReader("<!DOCTYPE html>")))

	a.MinifierEnabled = false

	req, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes 1-0")
	res.Header.Set(
		"Last-Modified",
		time.Unix(0, 0).UTC().Format(http.TimeFormat),
	)

	assert.Error(t, res.Write(strings.NewReader("foobar")))

	_, res, _ = fakeRRCycle(a, http.MethodGet, "/", nil)
	res.Status = http.StatusInternalServerError
	res.Header.Set("Content-Type", "text/plain; charset=utf-8")

	assert.Error(t, res.Write(&seekEndErrorSeeker{
		Reader: strings.NewReader("foobar"),
	}))
	assert.Error(t, res.Write(&seekStartErrorSeeker{
		Reader: strings.NewReader("foobar"),
	}))
	assert.NoError(t, res.Write(strings.NewReader("foobar")))

	_, res, _ = fakeRRCycle(a, http.MethodHead, "/", nil)
	res.Status = http.StatusInternalServerError
	res.Header.Set("Content-Type", "text/plain; charset=utf-8")

	assert.NoError(t, res.Write(strings.NewReader("foobar")))
}

func TestResponseWriteString(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.WriteString("foobar"))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"text/plain; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "foobar", string(hrwrb))
}

func TestResponseWriteHTML(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.WriteHTML("<!DOCTYPE html>"))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"text/html; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "<!DOCTYPE html>", string(hrwrb))
}

func TestResponseWriteJSON(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	var foobar struct {
		Foo string `json:"foo"`
	}
	foobar.Foo = "bar"

	assert.Error(t, res.WriteJSON(&errorJSONMarshaler{}))
	assert.NoError(t, res.WriteJSON(&foobar))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/json; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, `{"foo":"bar"}`, string(hrwrb))

	_, res, hrw = fakeRRCycle(a, http.MethodGet, "/", nil)

	a.DebugMode = true

	assert.Error(t, res.WriteJSON(&errorJSONMarshaler{}))
	assert.NoError(t, res.WriteJSON(&foobar))

	hrwr = hrw.Result()
	hrwrb, _ = ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/json; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "{\n\t\"foo\": \"bar\"\n}", string(hrwrb))
}

func TestResponseWriteXML(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	var foobar struct {
		XMLName xml.Name `xml:"foobar"`
		Foo     string   `xml:"foo"`
	}
	foobar.Foo = "bar"

	assert.Error(t, res.WriteXML(&errorXMLMarshaler{}))
	assert.NoError(t, res.WriteXML(&foobar))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/xml; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(
		t,
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<foobar><foo>bar</foo></foobar>",
		string(hrwrb),
	)

	_, res, hrw = fakeRRCycle(a, http.MethodGet, "/", nil)

	a.DebugMode = true

	assert.Error(t, res.WriteXML(&errorXMLMarshaler{}))
	assert.NoError(t, res.WriteXML(&foobar))

	hrwr = hrw.Result()
	hrwrb, _ = ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/xml; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(
		t,
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<foobar>\n\t<foo>bar</foo>\n</foobar>",
		string(hrwrb),
	)
}

func TestResponseWriteProtobuf(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.WriteProtobuf(&wrapperspb.StringValue{
		Value: "foobar",
	}))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/protobuf",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "\n\x06foobar", string(hrwrb))
}

func TestResponseWriteMsgpack(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	var foobar struct {
		Foo string `msgpack:"foo"`
	}
	foobar.Foo = "bar"

	assert.Error(t, res.WriteMsgpack(&errorMsgpackMarshaler{}))
	assert.NoError(t, res.WriteMsgpack(&foobar))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/msgpack",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "\x81\xa3foo\xa3bar", string(hrwrb))
}

func TestResponseWriteTOML(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	var foobar struct {
		Foo string `toml:"foo"`
	}
	foobar.Foo = "bar"

	assert.Error(t, res.WriteTOML(""))
	assert.NoError(t, res.WriteTOML(&foobar))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/toml; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "foo = \"bar\"\n", string(hrwrb))
}

func TestResponseWriteYAML(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	var foobar struct {
		Foo string `yaml:"foo"`
	}
	foobar.Foo = "bar"

	assert.Error(t, res.WriteYAML(&errorYAMLMarshaler{}))
	assert.NoError(t, res.WriteYAML(&foobar))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"application/yaml; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, "foo: bar\n", string(hrwrb))
}

func TestResponseRender(t *testing.T) {
	a := New()

	dir, err := ioutil.TempDir("", "air.TestResponseRender")
	assert.NoError(t, err)
	assert.NotEmpty(t, dir)
	defer os.RemoveAll(dir)

	a.RendererTemplateRoot = dir

	assert.NoError(t, ioutil.WriteFile(
		filepath.Join(a.RendererTemplateRoot, "test.html"),
		[]byte(`<a href="/">Go Home</a>`),
		os.ModePerm,
	))

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.Error(t, res.Render(nil, "foobar.html"))
	assert.NoError(t, res.Render(nil, "test.html"))

	hrwr := hrw.Result()
	hrwrb, _ := ioutil.ReadAll(hrwr.Body)

	assert.Equal(t, http.StatusOK, hrwr.StatusCode)
	assert.Equal(
		t,
		"text/html; charset=utf-8",
		hrw.HeaderMap.Get("Content-Type"),
	)
	assert.Equal(t, `<a href="/">Go Home</a>`, string(hrwrb))
}

func TestResponseRedihrwt(t *testing.T) {
	a := New()

	_, res, hrw := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.NoError(t, res.Redirect("http://example.com/foo/bar"))

	hrwr := hrw.Result()

	assert.Equal(t, http.StatusFound, hrwr.StatusCode)
	assert.Equal(
		t,
		"http://example.com/foo/bar",
		hrw.HeaderMap.Get("Location"),
	)
}

func TestResponseDefer(t *testing.T) {
	a := New()

	_, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)

	res.Defer(nil)
	assert.Len(t, res.deferredFuncs, 0)

	res.Defer(func() {})
	assert.Len(t, res.deferredFuncs, 1)
}

func TestResponseGzippable(t *testing.T) {
	a := New()

	req, res, _ := fakeRRCycle(a, http.MethodGet, "/", nil)

	assert.False(t, res.gzippable())

	req.Header.Set("Accept-Encoding", "gzip")
	assert.True(t, res.gzippable())

	req.Header.Set("Accept-Encoding", "br")
	assert.False(t, res.gzippable())

	req.Header.Set("Accept-Encoding", "gzip, br")
	assert.True(t, res.gzippable())

	req.Header.Set("Accept-Encoding", "br, gzip")
	assert.True(t, res.gzippable())

	req.Header.Set("Accept-Encoding", "br;q=1.0, gzip;q=0.8, *;q=0.1")
	assert.True(t, res.gzippable())
}

func TestNewReverseProxyBufferPool(t *testing.T) {
	rpbp := newReverseProxyBufferPool()

	assert.NotNil(t, rpbp.pool)
}

func TestReverseProxyBufferPoolGet(t *testing.T) {
	rpbp := newReverseProxyBufferPool()

	assert.Len(t, rpbp.Get(), 32<<20)
}

func TestReverseProxyBufferPoolPut(t *testing.T) {
	rpbp := newReverseProxyBufferPool()

	rpbp.Put(make([]byte, 32<<20))
}

type nopResponseWriter struct {
	http.ResponseWriter
}

func (nrw *nopResponseWriter) WriteHeader(int) {
}

func (nrw *nopResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

type readErrorReader struct {
	io.Seeker
}

func (rer *readErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

type seekErrorSeeker struct {
	io.Reader
}

func (ses *seekErrorSeeker) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek error")
}

type seekStartErrorSeeker struct {
	io.Reader
}

func (sses *seekStartErrorSeeker) Seek(_ int64, whence int) (int64, error) {
	if whence == io.SeekStart {
		return 0, errors.New("seek start error")
	}

	return 0, nil
}

type seekEndErrorSeeker struct {
	io.Reader
}

func (sees *seekEndErrorSeeker) Seek(_ int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 0, errors.New("seek end error")
	}

	return 0, nil
}

type errorJSONMarshaler struct {
}

func (ejm *errorJSONMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal json error")
}

type errorXMLMarshaler struct {
}

func (exm *errorXMLMarshaler) MarshalXML(*xml.Encoder, xml.StartElement) error {
	return errors.New("marshal xml error")
}

type errorMsgpackMarshaler struct {
}

func (emm *errorMsgpackMarshaler) MarshalMsgpack() ([]byte, error) {
	return nil, errors.New("marshal msgpack error")
}

type errorYAMLMarshaler struct {
}

func (eym *errorYAMLMarshaler) MarshalYAML() (interface{}, error) {
	return nil, errors.New("marshal yaml error")
}
