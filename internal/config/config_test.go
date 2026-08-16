package config

import "testing"

func TestNormalizeRejectsLongDiaryWindow(t *testing.T){c:=Default();c.Token="t";c.Secret="s";c.DiaryWindowText="32h";if err:=c.normalize();err!=nil{t.Fatal(err)};c.DiaryWindowText="800h";if err:=c.normalize();err==nil{t.Fatal("expected error")}}

