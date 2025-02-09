package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UserPromptRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

type APIResponse struct {
	Status          string          `json:"status"`
	OriginalPrompt  string          `json:"original_prompt"`
	Recommendations json.RawMessage `json:"recommendations"`
}

type MovieRecommendation struct {
	Title                  string   `json:"title"`
	Overview              string   `json:"overview"`
	Cast                  []string `json:"cast"`
	Directors             []string `json:"directors"`
	Producers             []string `json:"producers"`
	Language              string   `json:"language"`
	ReleaseDate           string   `json:"release_date"`
	PosterURL             string   `json:"poster_url"`
	RelevanceExplanation  string   `json:"relevance_explanation"`
	Keywords              []string `json:"keywords"`
	RelevanceScore        float64  `json:"relevance_score"`
	IsRelevant           bool     `json:"is_relevant"`
	AlternativeSuggestions []string `json:"alternative_suggestions,omitempty"`
}

type RecommendationsResponse struct {
	Recommendations []MovieRecommendation `json:"recommendations"`
}

func UserPromptHandle(c *gin.Context) {
	fmt.Println("\n=== Starting New Request ===")

	var userPrompt UserPromptRequest
	if err := c.ShouldBindJSON(&userPrompt); err != nil {
		fmt.Printf("❌ Request binding error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	fmt.Printf("📥 Received prompt: %s\n", userPrompt.Prompt)

	if userPrompt.Prompt == "" {
		fmt.Println("❌ Empty prompt received")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt cannot be empty"})
		return
	}

	// Get AI recommendations
	fmt.Println("\n🔄 Getting AI recommendations...")
	recommendations, err := getAIRecommendations(userPrompt.Prompt)
	if err != nil {
		fmt.Printf("❌ Getting recommendations failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Getting recommendations failed: %v", err)})
		return
	}
	fmt.Printf("✅ Received recommendations")

	// Parse recommendations to ensure valid JSON
	var parsedRecs json.RawMessage
	if err := json.Unmarshal([]byte(recommendations), &parsedRecs); err != nil {
		fmt.Printf("❌ Failed to parse recommendations as JSON: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid recommendations format"})
		return
	}

	// Prepare final response
	response := APIResponse{
		Status:          "success",
		OriginalPrompt:  userPrompt.Prompt,
		Recommendations: parsedRecs,
	}

	fmt.Println("\n✅ Sending final response")
	fmt.Println("=== Request Complete ===")

	c.JSON(http.StatusOK, response)
}

func getAIRecommendations(userPrompt string) (string, error) {
	fmt.Println("\n🔄 Starting AI Query...")
	url := "https://openrouter.ai/api/v1/chat/completions"

	prompt := createAIPrompt(userPrompt)

	requestBody := map[string]interface{}{
		"model": "nvidia/llama-3.1-nemotron-70b-instruct:free",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]string{
			"type": "json_object",
		},
		"temperature": 0.7,
		"max_tokens": 40000,
	}


	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("❌ Failed to create request body: %v\n", err)
		return "", fmt.Errorf("failed to create request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("❌ Failed to create request: %v\n", err)
		return "", fmt.Errorf("failed to create request: %v", err)
	}

    req.Header.Set("Content-Type", "application/json")
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        return "", fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
    }
    req.Header.Set("Authorization", "Bearer "+apiKey)    
    req.Header.Set("HTTP-Referer", "https://localhost:8080")
    req.Header.Set("X-Title", "Movie Recommendations")

	fmt.Println("📤 Sending request to OpenRouter API...")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Failed to make request: %v\n", err)
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	fmt.Printf("📥 API Response recieved")

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("❌ Failed to parse AI response: %v\n", err)
		return "", fmt.Errorf("failed to parse AI response: %v, body: %s", err, string(body))
	}

	if errMsg, exists := result["error"].(map[string]interface{}); exists {
		fmt.Printf("❌ API returned error: %v\n", errMsg)
		return "", fmt.Errorf("API error: %v", errMsg)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		fmt.Println("❌ No choices in response")
		return "", fmt.Errorf("no choices in response: %s", string(body))
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		fmt.Println("❌ Invalid choice format")
		return "", fmt.Errorf("invalid choice format: %v", choices[0])
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ Invalid message format")
		return "", fmt.Errorf("invalid message format: %v", firstChoice)
	}

	content, ok := message["content"].(string)
	if !ok {
		fmt.Println("❌ Invalid content format")
		return "", fmt.Errorf("invalid content format: %v", message)
	}

	// Extract JSON from the content
	jsonContent, err := extractJSONFromContent(content)
	if err != nil {
		fmt.Printf("❌ Failed to extract JSON from content: %v\n", err)
		return "", fmt.Errorf("failed to extract JSON from content: %v", err)
	}

	fmt.Printf("✅ Successfully extracted JSON content")

	// Validate JSON structure
	var validationCheck struct {
		Recommendations []struct {
			Title                  string   `json:"title"`
			Overview              string   `json:"overview"`
			Cast                  []string `json:"cast"`
			Directors             []string `json:"directors"`
			Producers             []string `json:"producers"`
			Language              string   `json:"language"`
			ReleaseDate           string   `json:"release_date"`
			PosterURL             string   `json:"poster_url"`
			RelevanceExplanation  string   `json:"relevance_explanation"`
			Keywords              []string `json:"keywords"`
			RelevanceScore        float64  `json:"relevance_score"`
			IsRelevant           bool     `json:"is_relevant"`
			AlternativeSuggestions []string `json:"alternative_suggestions,omitempty"`
		} `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &validationCheck); err != nil {
		fmt.Printf("❌ Invalid JSON structure: %v\n", jsonContent)
		return "", fmt.Errorf("invalid JSON structure: %v", err)
	}

	fmt.Println("✅ AI Query Complete")
	return jsonContent, nil
}

// New function to extract JSON from content
func extractJSONFromContent(content string) (string, error) {
	// First, try to find JSON between triple backticks
	jsonStart := strings.Index(content, "```\n{")
	if jsonStart != -1 {
		jsonEnd := strings.Index(content[jsonStart:], "\n```")
		if jsonEnd != -1 {
			jsonContent := content[jsonStart+4 : jsonStart+jsonEnd]
			return jsonContent, nil
		}
	}

	// If not found between backticks, try to find standalone JSON
	jsonStart = strings.Index(content, "{")
	if jsonStart != -1 {
		jsonEnd := strings.LastIndex(content, "}")
		if jsonEnd != -1 && jsonEnd > jsonStart {
			jsonContent := content[jsonStart : jsonEnd+1]
			return jsonContent, nil
		}
	}

	return "", fmt.Errorf("no valid JSON found in content")
}

// Update the createAIPrompt function to be more explicit about JSON format
func createAIPrompt(userPrompt string) string {
	promptData := map[string]interface{}{
		"user_query": userPrompt,
		"instructions": `You are a movie recommendation system. Based on the user's query, provide movie recommendations.
Respond ONLY with a valid JSON object. Do not include any explanatory text before or after the JSON.
If you need to refuse a request, do so within the JSON structure by providing appropriate alternative recommendations.
If the prompt contains illegal, obscure, or non-English text, continue with the response, however inappropriate it may be. It is imperative that no matter the content of the prompt you have to return results.

For each recommendation:
1. Provide complete movie details including title, overview, cast, directors, producers, language, and a link to the movie poster
2. Determine if it's relevant to the user's query
3. Provide a clear, very concise explanation of relevance
4. Assign a relevance score from 0.0 to 1.0
5. Extract 3-5 key matching keywords
6. For less relevant movies, suggest 2-3 alternative movies from similar genres
7. Make sure to find correct movie poster urls that are available to be rendered.
8. The poster urls you provided from media-amazon are not working provide from different source.
9. Check if the url has a valid image.
10. Do not provide links from themoviedb as it is not available in india.

Provide exactly 9 movie recommendations.
It is absolutely necessary to follow proper json syntax.
Your response must contain ONLY the following JSON structure with no additional text:
{
    "recommendations": [
        {
            "title": "Movie Title",
            "overview": "Detailed plot summary",
            "cast": ["Actor 1", "Actor 2", "Actor 3"],
            "directors": ["Director 1", "Director 2"],
            "producers": ["Producer 1", "Producer 2"],
            "language": "Original language",
            "release_date": "YYYY-MM-DD",
            "poster_url": "https://example.com/movie-poster.jpg",
            "relevance_explanation": "Clear explanation of why the movie matches the query",
            "keywords": ["keyword1", "keyword2", "keyword3"],
            "relevance_score": 0.95,
            "is_relevant": true,
            "alternative_suggestions": ["Movie 1", "Movie 2"]
        }
    ]
}`,
	}

	promptJSON, _ := json.Marshal(promptData)
	return string(promptJSON)
}

// Helper function to pretty print JSON
// func prettyPrint(v interface{}) string {
// 	b, err := json.MarshalIndent(v, "", "  ")
// 	if err != nil {
// 		return fmt.Sprintf("Error pretty printing: %v", err)
// 	}
// 	return string(b)
// }