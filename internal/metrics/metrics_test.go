package metrics

import "testing"

func TestWriterEscapesAndSorts(t *testing.T){w:=&Writer{};w.Sample("x",map[string]string{"z":"a\"b","a":"line\n"},1);want:="x{a=\"line\\n\",z=\"a\\\"b\"} 1\n";if w.String()!=want{t.Fatalf("got %q want %q",w.String(),want)}}

