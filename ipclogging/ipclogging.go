package ipclogging

import "log"

var DoDebug = false

func Print(v ...any) {
	log.Print(append([]any{"INFO "}, v...)...)
}
func Printf(format string, v ...any) {
	log.Printf("INFO  "+format, v...)
}
func Println(v ...any) {
	log.Println(append([]any{"INFO "}, v...)...)
}

func Debug(v ...any) {
	if DoDebug {
		log.Print(append([]any{"DEBUG"}, v...)...)
	}
}
func Debugf(format string, v ...any) {
	if DoDebug {
		log.Printf("DEBUG "+format, v...)
	}
}
func Debugln(v ...any) {
	if DoDebug {
		log.Println(append([]any{"DEBUG"}, v...)...)
	}
}
