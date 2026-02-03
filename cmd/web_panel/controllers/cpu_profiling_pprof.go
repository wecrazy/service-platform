package controllers

import (
	"net/http"
	"net/http/pprof"
)

func PprofIndex(w http.ResponseWriter, r *http.Request)   { pprof.Index(w, r) }
func PprofHeap(w http.ResponseWriter, r *http.Request)    { pprof.Handler("heap").ServeHTTP(w, r) }
func PprofProfile(w http.ResponseWriter, r *http.Request) { pprof.Profile(w, r) }
func PprofBlock(w http.ResponseWriter, r *http.Request)   { pprof.Handler("block").ServeHTTP(w, r) }
func PprofGoroutine(w http.ResponseWriter, r *http.Request) {
	pprof.Handler("goroutine").ServeHTTP(w, r)
}
func PprofThreadcreate(w http.ResponseWriter, r *http.Request) {
	pprof.Handler("threadcreate").ServeHTTP(w, r)
}
func PprofCmdline(w http.ResponseWriter, r *http.Request) { pprof.Cmdline(w, r) }
func PprofSymbol(w http.ResponseWriter, r *http.Request)  { pprof.Symbol(w, r) }
func PprofTrace(w http.ResponseWriter, r *http.Request)   { pprof.Trace(w, r) }
func PprofAllocs(w http.ResponseWriter, r *http.Request)  { pprof.Handler("allocs").ServeHTTP(w, r) }
func PprofMutex(w http.ResponseWriter, r *http.Request)   { pprof.Handler("mutex").ServeHTTP(w, r) }
