package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"mime"
	"mime/multipart"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type AWSService struct {
	s3Client *s3.Client
	bucket   string
}

func NewAWSService() (*AWSService, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	bucket := os.Getenv("S3_BUCKET_NAME")

	if accessKey == "" || secretKey == "" || region == "" || bucket == "" {
		return nil, fmt.Errorf("variables d'environnement AWS manquantes")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("erreur de configuration AWS: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	return &AWSService{
		s3Client: s3Client,
		bucket:   bucket,
	}, nil
}

// UploadBase64Images gère le téléchargement de plusieurs images base64 vers S3.
func (a *AWSService) UploadBase64Images(base64Images []string) ([]string, error) {
	// Add this log statement to see if the function received images
	log.Printf("Received %d images to upload", len(base64Images))

	var imageUrls []string

	for _, base64Image := range base64Images {
		// Décoder l'image base64
		imageData, err := base64.StdEncoding.DecodeString(base64Image)
		if err != nil {
			log.Printf("Erreur lors du décodage base64: %v", err)
			continue
		}

		// Détecter le type MIME de l'image
		contentType := detectImageContentType(imageData)
		if contentType == "" {
			log.Printf("Type d'image non supporté")
			continue
		}

		// Générer un nom de fichier unique
		fileName := fmt.Sprintf("chat-images/%s%s", uuid.New().String(), getFileExtension(contentType))

		// Upload vers S3
		imageUrl, err := a.uploadToS3(fileName, imageData, contentType)
		if err != nil {
			log.Printf("Erreur lors de l'upload vers S3: %v", err)
			continue
		}

		imageUrls = append(imageUrls, imageUrl)
	}

	return imageUrls, nil
}

// UploadAvatar gère le téléchargement d'un seul avatar base64 vers S3.
func (a *AWSService) UploadAvatar(base64Image string) (string, error) {
	if base64Image == "" {
		return "", fmt.Errorf("données d'image base64 vides")
	}

	// Décoder l'image base64
	imageData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		log.Printf("Erreur lors du décodage base64: %v", err)
		return "", fmt.Errorf("données base64 invalides")
	}

	// Détecter le type MIME de l'image
	contentType := detectImageContentType(imageData)
	if contentType == "" {
		log.Printf("Type d'image non supporté")
		return "", fmt.Errorf("type d'image non supporté")
	}

	// Générer un nom de fichier unique dans un dossier 'avatars'
	fileName := fmt.Sprintf("avatars/%s%s", uuid.New().String(), getFileExtension(contentType))

	// Upload vers S3
	imageUrl, err := a.uploadToS3(fileName, imageData, contentType)
	if err != nil {
		log.Printf("Erreur lors de l'upload de l'avatar vers S3: %v", err)
		return "", fmt.Errorf("échec de l'upload vers S3")
	}

	return imageUrl, nil
}

func (a *AWSService) uploadToS3(fileName string, data []byte, contentType string) (string, error) {
	_, err := a.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("erreur lors de l'upload S3: %v", err)
	}

	// Construire l'URL publique
	imageUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		a.bucket, os.Getenv("AWS_REGION"), fileName)

	return imageUrl, nil
}

func detectImageContentType(data []byte) string {
	// Vérifier les signatures de fichiers (magic numbers)
	if len(data) < 4 {
		return ""
	}

	// JPEG
	if bytes.Equal(data[:2], []byte{0xFF, 0xD8}) {
		return "image/jpeg"
	}

	// PNG
	if bytes.Equal(data[:4], []byte{0x89, 0x50, 0x4E, 0x47}) {
		return "image/png"
	}

	// GIF
	if bytes.Equal(data[:3], []byte{0x47, 0x49, 0x46}) {
		return "image/gif"
	}

	// WebP
	if len(data) >= 12 && bytes.Equal(data[:4], []byte{0x52, 0x49, 0x46, 0x46}) &&
		bytes.Equal(data[8:12], []byte{0x57, 0x45, 0x42, 0x50}) {
		return "image/webp"
	}

	return ""
}

// DeleteImages supprime plusieurs images de S3 en utilisant leurs URLs
func (a *AWSService) DeleteImages(imageUrls []string) error {
	region := os.Getenv("AWS_REGION")
	bucketName := a.bucket

	for _, imageUrl := range imageUrls {
		// Extraire la clé (nom du fichier) depuis l'URL
		key := extractKeyFromUrl(imageUrl, bucketName, region)
		if key == "" {
			log.Printf("Impossible d'extraire la clé depuis l'URL: %s", imageUrl)
			continue
		}

		// Supprimer l'objet de S3
		_, err := a.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})

		if err != nil {
			log.Printf("Erreur lors de la suppression de l'image %s: %v", imageUrl, err)
			// Continuer même si une image ne peut pas être supprimée
			continue
		}

		log.Printf("Image supprimée avec succès: %s", imageUrl)
	}

	return nil
}

// extractKeyFromUrl extrait la clé (nom du fichier) depuis une URL S3
func extractKeyFromUrl(imageUrl, bucketName, region string) string {
	// Format attendu: https://bucketname.s3.region.amazonaws.com/path/to/file.jpg
	expectedPrefix := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/", bucketName, region)

	if strings.HasPrefix(imageUrl, expectedPrefix) {
		return strings.TrimPrefix(imageUrl, expectedPrefix)
	}

	// Format alternatif: https://s3.region.amazonaws.com/bucketname/path/to/file.jpg
	alternativePrefix := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/", region, bucketName)
	if strings.HasPrefix(imageUrl, alternativePrefix) {
		return strings.TrimPrefix(imageUrl, alternativePrefix)
	}

	log.Printf("Format d'URL non reconnu: %s", imageUrl)
	return ""
}

// UploadAdImages gère le téléchargement de plusieurs images pour les annonces vers S3.
// Les images sont stockées dans le dossier 'ads/'
func (a *AWSService) UploadAdImages(base64Images []string) ([]string, error) {
	log.Printf("Réception de %d images d'annonce à uploader", len(base64Images))

	var imageUrls []string

	for i, base64Image := range base64Images {
		// Décoder l'image base64
		imageData, err := base64.StdEncoding.DecodeString(base64Image)
		if err != nil {
			log.Printf("Erreur lors du décodage base64 de l'image %d: %v", i+1, err)
			continue
		}

		// Détecter le type MIME de l'image
		contentType := detectImageContentType(imageData)
		if contentType == "" {
			log.Printf("Type d'image non supporté pour l'image %d", i+1)
			continue
		}

		// Générer un nom de fichier unique dans le dossier 'ads'
		fileName := fmt.Sprintf("ads/%s%s", uuid.New().String(), getFileExtension(contentType))

		// Upload vers S3
		imageUrl, err := a.uploadToS3(fileName, imageData, contentType)
		if err != nil {
			log.Printf("Erreur lors de l'upload de l'image %d vers S3: %v", i+1, err)
			continue
		}

		log.Printf("Image %d uploadée avec succès: %s", i+1, imageUrl)
		imageUrls = append(imageUrls, imageUrl)
	}

	if len(imageUrls) == 0 {
		return nil, fmt.Errorf("aucune image n'a pu être uploadée")
	}

	log.Printf("Total de %d images uploadées avec succès sur %d", len(imageUrls), len(base64Images))
	return imageUrls, nil
}

// UploadAdMultipartImages gère le téléchargement d'images multipart pour les annonces
func (a *AWSService) UploadAdMultipartImages(files []*multipart.FileHeader) ([]string, error) {
	log.Printf("Réception de %d fichiers multipart à uploader", len(files))

	var imageUrls []string

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("Erreur lors de l'ouverture du fichier %d: %v", i+1, err)
			continue
		}
		defer file.Close()

		// Lire le contenu du fichier
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(file)
		if err != nil {
			log.Printf("Erreur lors de la lecture du fichier %d: %v", i+1, err)
			continue
		}
		imageData := buf.Bytes()

		// Détecter le type MIME
		contentType := detectImageContentType(imageData)
		if contentType == "" {
			log.Printf("Type d'image non supporté pour le fichier %d", i+1)
			continue
		}

		// Générer un nom de fichier unique dans le dossier 'ads'
		fileName := fmt.Sprintf("ads/%s%s", uuid.New().String(), getFileExtension(contentType))

		// Upload vers S3
		imageUrl, err := a.uploadToS3(fileName, imageData, contentType)
		if err != nil {
			log.Printf("Erreur lors de l'upload du fichier %d vers S3: %v", i+1, err)
			continue
		}

		log.Printf("Fichier %d uploadé avec succès: %s", i+1, imageUrl)
		imageUrls = append(imageUrls, imageUrl)
	}

	if len(imageUrls) == 0 {
		return nil, fmt.Errorf("aucune image n'a pu être uploadée")
	}

	log.Printf("Total de %d fichiers uploadés avec succès sur %d", len(imageUrls), len(files))
	return imageUrls, nil
}

func getFileExtension(contentType string) string {
	ext, err := mime.ExtensionsByType(contentType)
	if err != nil || len(ext) == 0 {
		switch contentType {
		case "image/jpeg":
			return ".jpg"
		case "image/png":
			return ".png"
		case "image/gif":
			return ".gif"
		case "image/webp":
			return ".webp"
		default:
			return ".jpg"
		}
	}
	return ext[0]
}
