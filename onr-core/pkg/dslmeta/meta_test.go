package dslmeta

import "testing"

func TestMetaRequestRootJSONCached(t *testing.T) {
	meta := &Meta{
		RequestContentType: "application/json",
		RequestBody:        []byte(`{"model":"gpt-4o-mini","n":3}`),
	}

	root1 := meta.RequestRoot()
	if root1 == nil {
		t.Fatalf("expected request root")
	}
	if got, want := root1["model"], "gpt-4o-mini"; got != want {
		t.Fatalf("model got %v, want %v", got, want)
	}
	if got, want := root1["n"], float64(3); got != want {
		t.Fatalf("n got %v, want %v", got, want)
	}

	root1["cached"] = "yes"
	root2 := meta.RequestRoot()
	if root2 == nil {
		t.Fatalf("expected cached request root")
	}
	if got, want := root2["cached"], "yes"; got != want {
		t.Fatalf("cached marker got %v, want %v", got, want)
	}
}

func TestMetaRequestRootMultipartCached(t *testing.T) {
	meta := &Meta{
		RequestContentType: "multipart/form-data; boundary=abc123",
		RequestBody: []byte(
			"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"prompt\"\r\n\r\n" +
				"draw a cat\r\n" +
				"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"tag\"\r\n\r\n" +
				"a\r\n" +
				"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"tag\"\r\n\r\n" +
				"b\r\n" +
				"--abc123--\r\n",
		),
	}

	root1 := meta.RequestRoot()
	if root1 == nil {
		t.Fatalf("expected multipart request root")
	}
	if got, want := root1["prompt"], "draw a cat"; got != want {
		t.Fatalf("prompt got %v, want %v", got, want)
	}
	tags, ok := root1["tag"].([]any)
	if !ok {
		t.Fatalf("tag type=%T, want []any", root1["tag"])
	}
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("tags got %v, want [a b]", tags)
	}

	root1["cached"] = "yes"
	root2 := meta.RequestRoot()
	if got, want := root2["cached"], "yes"; got != want {
		t.Fatalf("cached marker got %v, want %v", got, want)
	}
}
