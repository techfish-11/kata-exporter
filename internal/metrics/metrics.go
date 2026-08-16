package metrics

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Writer struct { bytes.Buffer }

func (w *Writer) Help(name, help, typ string) { fmt.Fprintf(&w.Buffer,"# HELP %s %s\n# TYPE %s %s\n",name,escapeHelp(help),name,typ) }
func (w *Writer) Sample(name string, labels map[string]string, value float64) {
	w.WriteString(name)
	if len(labels)>0 {
		keys:=make([]string,0,len(labels)); for k:=range labels{keys=append(keys,k)}; sort.Strings(keys)
		w.WriteByte('{'); for i,k:=range keys { if i>0{w.WriteByte(',')}; fmt.Fprintf(&w.Buffer,"%s=\"%s\"",k,escapeLabel(labels[k])) }; w.WriteByte('}')
	}
	w.WriteByte(' '); w.WriteString(strconv.FormatFloat(value,'g',-1,64)); w.WriteByte('\n')
}
func escapeHelp(s string) string { return strings.NewReplacer("\\","\\\\","\n","\\n").Replace(s) }
func escapeLabel(s string) string { return strings.NewReplacer("\\","\\\\","\n","\\n","\"","\\\"").Replace(s) }

