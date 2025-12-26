package controllers

import (
	"net/http"
	"net/http/pprof"
)

// PprofIndex godoc
// @Summary      Pprof Index
// @Description  Returns the pprof index page
// @Tags         Pprof
// @Produce      html
// @Success      200  {string}   string "HTML"
// @Router       /debug/pprof/ [get]
func PprofIndex(w http.ResponseWriter, r *http.Request) { pprof.Index(w, r) }

// PprofHeap godoc
// @Summary      Pprof Heap
// @Description  Returns the pprof heap profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/heap [get]
func PprofHeap(w http.ResponseWriter, r *http.Request) { pprof.Handler("heap").ServeHTTP(w, r) }

// PprofProfile godoc
// @Summary      Pprof Profile
// @Description  Returns the pprof cpu profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/profile [get]
func PprofProfile(w http.ResponseWriter, r *http.Request) { pprof.Profile(w, r) }

// PprofBlock godoc
// @Summary      Pprof Block
// @Description  Returns the pprof block profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/block [get]
func PprofBlock(w http.ResponseWriter, r *http.Request) { pprof.Handler("block").ServeHTTP(w, r) }

// PprofGoroutine godoc
// @Summary      Pprof Goroutine
// @Description  Returns the pprof goroutine profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/goroutine [get]
func PprofGoroutine(w http.ResponseWriter, r *http.Request) {
	pprof.Handler("goroutine").ServeHTTP(w, r)
}

// PprofThreadcreate godoc
// @Summary      Pprof Threadcreate
// @Description  Returns the pprof threadcreate profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/threadcreate [get]
func PprofThreadcreate(w http.ResponseWriter, r *http.Request) {
	pprof.Handler("threadcreate").ServeHTTP(w, r)
}

// PprofCmdline godoc
// @Summary      Pprof Cmdline
// @Description  Returns the pprof cmdline arguments
// @Tags         Pprof
// @Success      200  {string}   string
// @Router       /debug/pprof/cmdline [get]
func PprofCmdline(w http.ResponseWriter, r *http.Request) { pprof.Cmdline(w, r) }

// PprofSymbol godoc
// @Summary      Pprof Symbol
// @Description  Returns the pprof symbol information
// @Tags         Pprof
// @Success      200  {string}   string
// @Router       /debug/pprof/symbol [get]
func PprofSymbol(w http.ResponseWriter, r *http.Request) { pprof.Symbol(w, r) }

// PprofTrace godoc
// @Summary      Pprof Trace
// @Description  Returns the pprof trace profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/trace [get]
func PprofTrace(w http.ResponseWriter, r *http.Request) { pprof.Trace(w, r) }

// PprofAllocs godoc
// @Summary      Pprof Allocs
// @Description  Returns the pprof allocs profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/allocs [get]
func PprofAllocs(w http.ResponseWriter, r *http.Request) { pprof.Handler("allocs").ServeHTTP(w, r) }

// PprofMutex godoc
// @Summary      Pprof Mutex
// @Description  Returns the pprof mutex profile
// @Tags         Pprof
// @Success      200  {file}     file
// @Router       /debug/pprof/mutex [get]
func PprofMutex(w http.ResponseWriter, r *http.Request) { pprof.Handler("mutex").ServeHTTP(w, r) }
