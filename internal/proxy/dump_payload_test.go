package proxy

import "testing"

func TestIsBinaryDumpPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		contentType string
		payload     []byte
		wantBinary  bool
	}{
		{
			name:        "text_event_stream_by_header",
			contentType: "text/event-stream",
			payload:     []byte("event: x\ndata: {}\n\n"),
			wantBinary:  false,
		},
		{
			name:        "octet_stream_by_header",
			contentType: "application/octet-stream",
			payload:     []byte{0x00, 0x01, 0x02},
			wantBinary:  true,
		},
		{
			name:        "missing_content_type_sse_text",
			contentType: "",
			payload:     []byte("event: response.created\ndata: {\"type\":\"response.created\"}\n\n"),
			wantBinary:  false,
		},
		{
			name:        "missing_content_type_binary",
			contentType: "",
			payload:     []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10},
			wantBinary:  true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isBinaryDumpPayload(tc.contentType, tc.payload)
			if got != tc.wantBinary {
				t.Fatalf("isBinaryDumpPayload()=%v want=%v", got, tc.wantBinary)
			}
		})
	}
}
