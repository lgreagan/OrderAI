package services

import "OrderAI/models"

type AIService interface {
	RecognizeOrder(imagePath string) (*models.AIOrderResult, error)
}
