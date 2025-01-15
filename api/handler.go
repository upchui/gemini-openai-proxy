package api

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/pkg/errors"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/zhu327/gemini-openai-proxy/pkg/adapter"
)

func IndexHandler(c *gin.Context) {
	c.JSON(http.StatusMisdirectedRequest, gin.H{
		"message": "Welcome to the OpenAI API! Documentation is available at https://platform.openai.com/docs/api-reference",
	})
}

func ModelListHandler(c *gin.Context) {
	// Retrieve the Authorization header value
	authorizationHeader := c.GetHeader("Authorization")
	var openaiAPIKey string
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	ctx := c.Request.Context()
	client, err := genai.NewClient(ctx, option.WithAPIKey(openaiAPIKey))
	if err != nil {
		log.Printf("Error creating genai client: %v\n", err)
		handleGenerateContentError(c, err)
		return
	}
	defer client.Close()

	iter := client.ListModels(ctx)

	var modelList []openai.Model
	for {
		modelInfo, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			handleGenerateContentError(c, err)
			return
		}
		openaiModel := openai.Model{
			ID:      modelInfo.Name,
			Object:  "model",
			OwnedBy: "google",
		}
		modelList = append(modelList, openaiModel)
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   modelList,
	})
}

func ModelRetrieveHandler(c *gin.Context) {
	modelID := c.Param("model")
	authorizationHeader := c.GetHeader("Authorization")
	var openaiAPIKey string
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	ctx := c.Request.Context()
	client, err := genai.NewClient(ctx, option.WithAPIKey(openaiAPIKey))
	if err != nil {
		log.Printf("Error creating genai client: %v\n", err)
		handleGenerateContentError(c, err)
		return
	}
	defer client.Close()

	// Iterate over models to find the requested model
	iter := client.ListModels(ctx)

	var foundModel *genai.ModelInfo
	for {
		modelInfo, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			handleGenerateContentError(c, err)
			return
		}
		if modelInfo.Name == modelID {
			foundModel = modelInfo
			break
		}
	}

	if foundModel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	openaiModel := openai.Model{
		ID:      foundModel.Name,
		Object:  "model",
		OwnedBy: "google",
	}

	c.JSON(http.StatusOK, openaiModel)
}

func ChatProxyHandler(c *gin.Context) {
	authorizationHeader := c.GetHeader("Authorization")
	var openaiAPIKey string
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	req := &adapter.ChatCompletionRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	messages, err := req.ToGenaiMessages()
	if err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	client, err := genai.NewClient(ctx, option.WithAPIKey(openaiAPIKey))
	if err != nil {
		log.Printf("Error creating genai client: %v\n", err)
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	defer client.Close()

	model := req.Model
	gemini := adapter.NewGeminiAdapter(client, model)

	if !req.Stream {
		resp, err := gemini.GenerateContent(ctx, req, messages)
		if err != nil {
			handleGenerateContentError(c, err)
			return
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	dataChan, err := gemini.GenerateStreamContent(ctx, req, messages)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	setEventStreamHeaders(c)
	c.Stream(func(w io.Writer) bool {
		if data, ok := <-dataChan; ok {
			c.Render(-1, adapter.Event{Data: "data: " + data})
			return true
		}
		c.Render(-1, adapter.Event{Data: "data: [DONE]"})
		return false
	})
}

func EmbeddingProxyHandler(c *gin.Context) {
	authorizationHeader := c.GetHeader("Authorization")
	var openaiAPIKey string
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	req := &adapter.EmbeddingRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	messages, err := req.ToGenaiMessages()
	if err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	client, err := genai.NewClient(ctx, option.WithAPIKey(openaiAPIKey))
	if err != nil {
		log.Printf("Error creating genai client: %v\n", err)
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	defer client.Close()

	model := req.Model
	gemini := adapter.NewGeminiAdapter(client, model)

	resp, err := gemini.GenerateEmbedding(ctx, messages)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func setEventStreamHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func handleGenerateContentError(c *gin.Context, err error) {
	log.Printf("genai generate content error %v\n", err)

	// Try OpenAI API error first
	var openaiErr *openai.APIError
	if errors.As(err, &openaiErr) {
		statusCode := http.StatusInternalServerError
		if code, ok := openaiErr.Code.(int); ok {
			statusCode = code
		}
		c.AbortWithStatusJSON(statusCode, openaiErr)
		return
	}

	// Try googleapi.Error
	var googleapiErr *googleapi.Error
	if errors.As(err, &googleapiErr) {
		log.Printf("Handling googleapi error with code: %d\n", googleapiErr.Code)
		statusCode := googleapiErr.Code
		if statusCode == http.StatusTooManyRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, openai.APIError{
				Code:    http.StatusTooManyRequests,
				Message: "Rate limit exceeded",
				Type:    "rate_limit_error",
			})
			return
		}
		c.AbortWithStatusJSON(statusCode, openai.APIError{
			Code:    statusCode,
			Message: googleapiErr.Message,
			Type:    "server_error",
		})
		return
	}

	// For all other errors
	log.Printf("Handling unknown error: %v\n", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, openai.APIError{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
		Type:    "server_error",
	})
}