# Frontend Integration

The Patchflow project uses a decoupled frontend architecture for development, but a bundled approach for production.

## Architecture

1. **Development (`patchflow-dashboard`)**:
   The dashboard UI is a React application built with Vite. It resides in the `patchflow-dashboard` repository/folder. Developers can run `npm run dev` to use Hot Module Replacement (HMR) and iterate quickly on the UI.

2. **Production (`dist/`)**:
   When the UI is ready, `npm run build` compiles the React application into static files (HTML, JS, CSS) inside the `patchflow/dist` folder. 
   
   The `dist/` folder is **intentionally committed to this core repository**. This ensures that the Go backend can always serve a working version of the dashboard without requiring Node.js or Vite to be installed in the production or build environments.

## Serving the Dashboard

To serve the dashboard from the Go backend, you can use Go's standard `http.FileServer`:

```go
package main

import (
	"log"
	"net/http"
)

func main() {
	// Serve the static files from the dist directory
	fs := http.FileServer(http.Dir("./dist"))
	http.Handle("/dashboard/", http.StripPrefix("/dashboard/", fs))

	log.Println("Dashboard available at http://localhost:8080/dashboard/")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

This ensures a seamless experience where the API proxy and the management dashboard are served from the same binary.
