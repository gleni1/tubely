package main

import (
	"net/http"
	"io"
	"mime"
  "context"
  "fmt"
	"bytes"
  "encoding/json"
  "os"
  "os/exec"
  "encoding/hex"
  "math/rand"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
  "github.com/aws/aws-sdk-go-v2/service/s3" 
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = int64(1 << 30) // 1 GB
  r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

  
  err = r.ParseMultipartForm(10 << 20)
  if err != nil {
    respondWithError(w, http.StatusBadRequest, "Unable to parse form", err)
    return
  }

  file, header, err := r.FormFile("video")
  if err != nil {
    respondWithError(w, http.StatusInternalServerError, "Error reading video file", err)
    return
  }
  defer file.Close()


	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}


  f, err := os.CreateTemp("", "tubely-upload.mp4")
  if err != nil {
    respondWithError(w, http.StatusInternalServerError, "Couldn't create temproary file", err)
    return
  }
  defer os.Remove(f.Name())
  defer f.Close()

  if _, err := io.Copy(f, file); err != nil {
    respondWithError(w, http.StatusInternalServerError, "Couldn't copy content from mulitple-part file to temp new file", err)
    return
  }

  if _, err := f.Seek(0, io.SeekStart); err != nil {
    respondWithError(w, http.StatusInternalServerError, "Error resetting file pointer", err)
    return
  }

  aspectRatio, err := getVideoAspectRatio(f.Name())
  if err != nil {
    respondWithError(w, http.StatusInternalServerError, "Error getting aspect ratio: ", err)
    return
  }


  keyBytes := make([]byte, 16)
  if _, err := rand.Read(keyBytes); err != nil {
    respondWithError(w, http.StatusInternalServerError, "Err: ", err)
    return
  }
  key := aspectRatio + "/" + hex.EncodeToString(keyBytes) + ".mp4"

  putObjectInput := &s3.PutObjectInput {
    Bucket:       strPtr(cfg.s3Bucket),
    Key:          &key,
    Body:         f,
    ContentType:  strPtr("video/mp4"),
  }

  _, err = cfg.s3Client.PutObject(context.TODO(), putObjectInput)
  if err != nil {
    respondWithError(w, http.StatusInternalServerError, "Failed to upload to S3:", err)
    return 
  }


  vidUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	video.VideoURL= &vidUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func strPtr(s string) *string {
  return &s
}

func getVideoAspectRatio(filePath string) (string, error) {
  cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
  var outBuffer bytes.Buffer 
  cmd.Stdout = &outBuffer 
  err := cmd.Run()
  if err != nil {
    return "", fmt.Errorf("error running ffprobe: %w", err)
  }

  var stream streamStruct 
  decoder := json.NewDecoder(&outBuffer)
  err = decoder.Decode(&stream)
  if err != nil {
    return "", fmt.Errorf("Error decoding the data into a json struct: %w", err)
  }

  var aspectRatio string
  ratio := float64(stream.Streams[0].Width) / float64(stream.Streams[0].Height)

  if ratio > 1.7 && ratio < 1.8 {
      aspectRatio = "landscape" 
  } else if ratio > 0.5 && ratio < 0.6 {
      aspectRatio = "portrait" 
  } else {
      aspectRatio = "other"
  }

  return aspectRatio, nil
}
