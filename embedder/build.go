package embedder

/*
// Linux Build Tags
// ----------------
#cgo linux CFLAGS: -I${SRCDIR}/library
#cgo linux LDFLAGS: -lflutter_engine -Wl,-rpath,$ORIGIN

// Windows Build Tags
// ----------------
#cgo windows CFLAGS: -I${SRCDIR}/library
#cgo windows LDFLAGS: -lflutter_engine

// Darwin Build Tags
// ----------------
#cgo darwin CFLAGS: -I${SRCDIR}/library
#cgo darwin LDFLAGS: -framework FlutterEmbedder

*/
import "C"
