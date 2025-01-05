package main

import (
	"fmt"
	"net/http"
  "io"
  "os"
  "mime"
  _ "strings"
  _ "encoding/base64"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)


func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

  const maxMemory = 10 << 20
  r.ParseMultipartForm(maxMemory)

  file, header, err := r.FormFile("thumbnail")
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
    return
  }
  defer file.Close()


//  var imgData []byte
//  imgData, err = io.ReadAll(file)
//  if err != nil {
//    respondWithError(w, http.StatusBadRequest, "Unable to read image data into slice", err)
//    return
//  }

//  media := strings.Split((header.Header["Content-Type"][0]), "/")[1]

  mediaType, _, err := mime.ParseMediaType(header.Header["Content-Type"][0])
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Error while parsing media type", err)
    return
  } 
  if mediaType != "image/jpeg" && mediaType != "image/png" {
    respondWithError(w, http.StatusBadRequest, "Media type not allowed", err)
    return
  }

  video, err := cfg.db.GetVideo(videoID)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Unable to get videos from db", err)
    return
  }

  if video.UserID != userID {
    respondWithError(w, http.StatusUnauthorized, "Current user is not the author of the videos", nil)
    return
  }

   
  filePath := fmt.Sprintf("assets/%s.%s", videoID, mediaType)
  newFile, err := os.Create(filePath)
  if err != nil {
    fmt.Println("Error creating file:", err)
    return
  }
  defer newFile.Close()

  if _, err := io.Copy(newFile, file); err != nil {
    fmt.Println("Error copying file content", err)
    return
  }


  newThumbnail := fmt.Sprintf("http://localhost:%d/%s", cfg.port, filePath)
  
  video.ThumbnailURL = &newThumbnail

  err = cfg.db.UpdateVideo(video)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Cannot update video", err)
    return
  }

	respondWithJSON(w, http.StatusOK, video)
}
