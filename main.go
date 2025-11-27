package main 

import (
	"fmt"
	//"log"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func main(){
	r := chi.NewRouter()                                    // Create a new chi router instance

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
  	http.ServeFile(w, r, "static/upload.html")
  })
	
	r.Post("/upload", uploadHandler)                        // Register  POST /upload endpoint and map it to uploadHandler
	fmt.Println("server running on http://localhost:8080")  // Log server start
	http.ListenAndServe("0.0.0.0:8080", r)                         // Start the HTTP server
}

func uploadHandler(w http.ResponseWriter, r *http.Request){
	r.ParseMultipartForm(50<<20)
	file, handler, err := r.FormFile("file")                // FormFile returns the file and the FileHeader(File Name, Size)
	if err != nil {
		http.Error(w, "File upload error", http.StatusBadRequest)
		return
	}
	defer file.Close()

	os.MkdirAll("data", os.ModePerm)											  // Ensure "data" folder exists 
	
	dst := filepath.Join("data", handler.Filename)          // Build the local file path
	out, err:= os.Create(dst)                               // Create a new file on your disk to save the upload
	if err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
    return
	}
	defer out.Close()                                       //We close files to avoid memory leaks, prevent errors, ensure data is saved, and free up system resources.                 

	_, err = io.Copy(out, file)                             // Copy uploaded file 
	if err != nil{                                         
		http.Error(w, "Error saving file", http.StatusInternalServerError)
  	return
	}

	fmt.Fprintf(w, "File uploaded successfully: %s", handler.Filename)
}





