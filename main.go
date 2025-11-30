package main 

import (
	"fmt"
	"log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"encoding/json"
	"strings"
	"net/url"
	"mime"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/go-chi/chi/v5/middleware"
)

func main(){	
	// Load Configs
	viper.SetConfigName("config") // config.yaml
  viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
    log.Fatalf("Error loading config: %v", err)
  }

	port := viper.GetInt("server.port")
	username := viper.GetString("auth.username")
	password := viper.GetString("auth.password")

	storagePath := viper.GetString("storage.path")

	fmt.Println("Storage Path:", storagePath)
  fmt.Printf("Server starting on port %d\n", port)

	r := chi.NewRouter() // Create a new chi router instance

	// Middleware
	r.Use(middleware.Logger)   // logs all HTTP requests
	r.Use(middleware.Recoverer) // recovers from panics

	
	r.Group(func(r chi.Router){
		r.Use(func(next http.Handler) http.Handler {
      return basicAuthMiddleware(next, username, password)
    })

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "static/upload.html")
		})

		r.Get("/list", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "static/files.html")
		}) 


		r.Post("/upload", func(w http.ResponseWriter, r *http.Request){
			uploadHandler(w, r, storagePath)
		}) // Register POST /upload endpoint and map it to uploadHandler
		
		r.Get("/files", func(w http.ResponseWriter, r *http.Request){
			listFileHandler(w, r, storagePath)
		}) // Register GET /files endpoint 

		r.Get("/files/{filename}", func(w http.ResponseWriter, r *http.Request) {
			downloadHandler(w, r, storagePath)
		}) // Register GET /files/{filename} download endpoint

		r.Delete("/files/{filename}", func(w http.ResponseWriter, r *http.Request){
			filename := chi.URLParam(r, "filename")
			log.Printf("filename:"+filename)
			deleteHandler(w, r, storagePath)
		})

		r.Get("/preview/{filename}", func(w http.ResponseWriter, r *http.Request) {
    	http.ServeFile(w, r, "static/preview.html")
		})

		r.Get("/preview/type/{filename}", func(w http.ResponseWriter, r *http.Request){
			previewTypeHandler(w, r, storagePath);
		})

		r.Get("/preview/raw/{filename}", func(w http.ResponseWriter, r *http.Request) {
    	rawPreviewHandler(w, r, storagePath)
		})

		r.Put("/files/{filename}", func(w http.ResponseWriter, r *http.Request){
			renameHandler(w, r, storagePath)
		})
	})
	fmt.Println("server running on http://localhost:8080")  // Log server start
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	http.ListenAndServe(addr, r)   // Start the HTTP server
}



func uploadHandler(w http.ResponseWriter, r *http.Request, storagePath string){
	r.ParseMultipartForm(50<<20)
	file, handler, err := r.FormFile("file") // FormFile returns the file and the FileHeader(File Name, Size)
	if err != nil {
		http.Error(w, "File upload error", http.StatusBadRequest)
		return
	}
	defer file.Close()

	os.MkdirAll(storagePath, os.ModePerm)	 // Ensure "data" folder exists 
	
	dst := filepath.Join(storagePath, handler.Filename) // Build the local file path
	out, err:= os.Create(dst)  // Create a new file on your disk to save the upload
	if err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
    return
	}
	defer out.Close()  //We close files to avoid memory leaks, prevent errors, ensure data is saved, and free up system resources.             
	_, err = io.Copy(out, file)   // Copy uploaded file 
	if err != nil{                                         
		http.Error(w, "Error saving file", http.StatusInternalServerError)
  	return
	}
  log.Printf("Uploaded file: %s (%d bytes)\n", handler.Filename, handler.Size)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	//fmt.Fprintf(w, "File uploaded successfully: %s", handler.Filename)
}



func listFileHandler(w http.ResponseWriter, r *http.Request, storagePath string){   
	files, err := os.ReadDir(storagePath)
	if err != nil {
		http.Error(w, "Unable to read files", http.StatusInternalServerError)
    return
	}
	w.Header().Set("Content-Type", "application/json")
	var list []string
    for _, f := range files {
        if !f.IsDir() {
            list = append(list, f.Name())
        }
    }
	json.NewEncoder(w).Encode(list)
}



func downloadHandler(w http.ResponseWriter, r *http.Request, storagePath string){
	filename := chi.URLParam(r, "filename")
	filePath := filepath.Join(storagePath, filename)
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
	  return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}



func deleteHandler(w http.ResponseWriter, r *http.Request, storagePath string){
	filename := chi.URLParam(r, "filename")
	// Decode URL encoding (%20 → space)
	filename, err := url.QueryUnescape(filename)
	if err != nil {
			http.Error(w, "Invalid filename encoding", http.StatusBadRequest)
			return
	}
	//Prevent path traversal attacks
	if strings.Contains(filename, ".."){
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(storagePath, filename)
	// Check if file exists 
	if _,err := os.Stat(filePath); os.IsNotExist(err){
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	// Remove File
	if err := os.Remove(filePath); err != nil {
		http.Error(w, "Unable to delete the file", http.StatusInternalServerError)
		return 
	}
	fmt.Fprintf(w, "File Deleted: %s", filename)
	log.Println("Deleted file", filename)
}



func basicAuthMiddleware(next http.Handler, username, password string) http.Handler{
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()

		if !ok || u != username || p != password{
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return 
		}

		next.ServeHTTP(w,r)
	})
}



func previewTypeHandler(w http.ResponseWriter, r *http.Request, storagePath string){
	encoded := chi.URLParam(r, "filename")
	filename, err := url.QueryUnescape(encoded)
	if err != nil{
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	//filePath := filepath.Join(storagePath, filename)
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == ""{
		mimeType = "application/octet-stream"
	}
	w.Write([] byte(mimeType))
}


func rawPreviewHandler(w http.ResponseWriter, r *http.Request, storagePath string) {
    encoded := chi.URLParam(r, "filename")
    filename, err := url.QueryUnescape(encoded)
    if err != nil {
        http.Error(w, "Invalid filename", http.StatusBadRequest)
        return
    }
    filePath := filepath.Join(storagePath, filename)
    mimeType := mime.TypeByExtension(filepath.Ext(filename))
    if mimeType == "" {
        mimeType = "application/octet-stream"
    }    
		w.Header().Set("Content-Type", mimeType)
    http.ServeFile(w, r, filePath)
}


func renameHandler(w http.ResponseWriter, r *http.Request, storagePath string){
	encodedOld := chi.URLParam(r, "filename")
	oldName, err := url.QueryUnescape(encodedOld)
	if err != nil {
		http.Error(w, "Invalid Old Filename", http.StatusBadRequest)
		return
	}

	newName := r.URL.Query().Get("new")
	if newName == "" {
			http.Error(w, "Missing new name", http.StatusBadRequest)
			return
	}

	// Basic security: prevent path traversal
	if strings.Contains(oldName, "..") || strings.Contains(newName, "..") {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
	}

	oldPath := filepath.Join(storagePath, oldName)
	newPath := filepath.Join(storagePath, newName)

	// Check if original exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			http.Error(w, "Original file not found", http.StatusNotFound)
			return
	}

	// Avoid overwriting
	if _, err := os.Stat(newPath); err == nil {
			http.Error(w, "New filename already exists", http.StatusConflict)
			return
	}

	// Perform rename
	if err := os.Rename(oldPath, newPath); err != nil {
			http.Error(w, "Rename failed", http.StatusInternalServerError)
			return
	}

	fmt.Fprintf(w, "Renamed to %s", newName) 
}
