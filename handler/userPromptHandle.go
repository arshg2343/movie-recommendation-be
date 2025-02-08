package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pinecone-io/go-pinecone/pinecone"
)

type UserPromptRequest struct {
    Prompt string `json:"prompt" binding:"required"` 
}

type APIResponse struct {
    Status          string          `json:"status"`
    OriginalPrompt  string          `json:"original_prompt"`
    OptimizedPrompt string          `json:"optimized_prompt"`
    Recommendations json.RawMessage `json:"recommendations"`
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

    // Optimize the prompt
    fmt.Println("\n🔄 Optimizing prompt...")
    optimizedPrompt, err := userPrompt.optimisePrompt(userPrompt.Prompt)
    if err != nil {
        fmt.Printf("❌ Prompt optimization failed: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Prompt optimization failed: %v", err)})
        return
    }
    fmt.Printf("✅ Optimized prompt: %s\n", optimizedPrompt)

    // Get embeddings
    fmt.Println("\n🔄 Generating embeddings...")
    embedding, err := getEmbedding(optimizedPrompt)
    if err != nil {
        fmt.Printf("❌ Embedding generation failed: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Embedding generation failed: %v", err)})
        return
    }
    fmt.Printf("✅ Generated embedding with length: %d\n", len(embedding))

    // Query Pinecone
    fmt.Println("\n🔄 Querying Pinecone...")
    pineconeResponse, err := queryPinecone(embedding)
    if err != nil {
        fmt.Printf("❌ Pinecone query failed: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Pinecone query failed: %v", err)})
        return
    }
    fmt.Printf("✅ Pinecone response received:\n%s\n", prettyPrint(pineconeResponse))

    // Process recommendations
    fmt.Println("\n🔄 Processing recommendations...")
    recommendations, err := processMovieRecommendations(userPrompt.Prompt, pineconeResponse)
    if err != nil {
        fmt.Printf("❌ Processing recommendations failed: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Processing recommendations failed: %v", err)})
        return
    }
    fmt.Printf("✅ Processed recommendations:\n%s\n", prettyPrint(recommendations))

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
        OptimizedPrompt: optimizedPrompt,
        Recommendations: parsedRecs,
    }

    fmt.Println("\n✅ Sending final response")
    fmt.Printf("Final Response:\n%s\n", prettyPrint(response))
    fmt.Println("=== Request Complete ===")

    c.JSON(http.StatusOK, response)
}

// Helper function to pretty print JSON
func prettyPrint(v interface{}) string {
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return fmt.Sprintf("Error pretty printing: %v", err)
    }
    return string(b)
}



type WitResponse struct {
	Entities map[string][]Entity `json:"entities"`
	Text     string              `json:"text"`
}

type Entity struct {
	Body string `json:"body"`
}

func (UserPrompt *UserPromptRequest) optimisePrompt(prompt string) (string, error) {
	encodedQuery := url.QueryEscape(prompt)
	apiURL := fmt.Sprintf("https://api.wit.ai/message?v=20250206&q=%s", encodedQuery)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "error in request", err
	}

	req.Header.Set("Authorization", "Bearer Q2G6R4GUMZ5P3IZKW245ESIC7V6X5R6E")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "error in sending request", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "error in reading", err
	}

	var witResponse WitResponse
	if err := json.Unmarshal(body, &witResponse); err != nil {
		return "error in unmarshalling", err
	}

	var entityBodies []string
	for _, entities := range witResponse.Entities {
		for _, entity := range entities {
			entityBodies = append(entityBodies, entity.Body)
		}
	}

	return strings.Join(entityBodies, " "), nil

}

func getEmbedding(text string) ([]float64, error) {
    cmd := exec.Command("./handler/embed.exe", text) // Path to the generated executable
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil {
        return nil, err
    }

    var embedding []float64
    err = json.Unmarshal(out.Bytes(), &embedding)
    if err != nil {
        return nil, err
    }

    return embedding, nil
}


func queryPinecone(embedding []float64) (string, error) {
    ctx := context.Background()

    clientParams := pinecone.NewClientParams{
        ApiKey: "pcsk_37vcFD_9hWdqWeAKBDJ2CpJQxTPCnbKYmyWCR9rrDhrm61PUD4urRQCUu5JS8aQvJhk5ZT",
    }

    pc, err := pinecone.NewClient(clientParams)
    if err != nil {
        return "", fmt.Errorf("failed to create Pinecone client: %v", err)
    }

	idx, err := pc.DescribeIndex(ctx, "movie-search")
    if err != nil {
        return "", fmt.Errorf("failed to describe index: %v", err)
    }

	if idx == nil {}

    idxConnection, err := pc.Index(pinecone.NewIndexConnParams{
        Host:      "https://movie-search-84j7gob.svc.aped-4627-b74a.pinecone.io",
        Namespace: "",
    })
    if err != nil {
        return "", fmt.Errorf("failed to create index connection: %v", err)
    }

    vector := make([]float32, len(embedding))
    for i, v := range embedding {
        vector[i] = float32(v)
    }

    queryResponse, err := idxConnection.QueryByVectorValues(ctx, &pinecone.QueryByVectorValuesRequest{
        Vector:         vector,
        TopK:          10,
        IncludeValues: false,
        IncludeMetadata: true,
    })
    if err != nil {
        return "", fmt.Errorf("failed to query index: %v", err)
    }

    if len(queryResponse.Matches) == 0 {
        return "", fmt.Errorf("no matches found")
    }

    responseJSON, err := json.Marshal(queryResponse)
    if err != nil {
        return "", fmt.Errorf("failed to marshal response: %v", err)
    }

    return string(responseJSON), nil
}


type PineconeResponse struct {
    Matches []struct {
        Vector struct {
            ID       string `json:"id"`
            Metadata struct {
                Title            string `json:"title"`
                OriginalTitle    string `json:"original_title"`
                Overview         string `json:"overview"`
                Genres          string `json:"genres"`
                ReleaseDate     string `json:"release_date"`
                SpokenLanguages string `json:"spoken_languages"`
                Tagline         string `json:"tagline"`
            } `json:"metadata"`
        } `json:"vector"`
        Score float64 `json:"score"`
    } `json:"matches"`
}

type MovieMatch struct {
    ID       string  `json:"id"`
    Title    string  `json:"title"`
    Overview string  `json:"overview"`
    Genres   string  `json:"genres"`
    Year     string  `json:"release_date"`
    Score    float64 `json:"score"`
}

type AIResponse struct {
    Recommendations []struct {
        Title       string   `json:"title"`
        Overview    string   `json:"overview"`
        Cast        []string `json:"cast"`
        Directors   []string `json:"directors"`
        Producers   []string `json:"producers"`
        Language    string   `json:"language"`
        ReleaseDate string   `json:"release_date"`
        PosterURL   string   `json:"poster_url"`
        Relevance   string   `json:"relevance_explanation"`
        Keywords    []string `json:"keywords"`
        Score       float64  `json:"relevance_score"`
        IsRelevant  bool     `json:"is_relevant"`
        Suggestions []string `json:"alternative_suggestions,omitempty"`
    } `json:"recommendations"`
}

func processMovieRecommendations(userPrompt string, pineconeResponseData string) (string, error) {
    var pineconeResp PineconeResponse
    err := json.Unmarshal([]byte(pineconeResponseData), &pineconeResp)
    if err != nil {
        return "", fmt.Errorf("failed to parse Pinecone response: %v", err)
    }

    var movieMatches []MovieMatch
    for _, match := range pineconeResp.Matches {
        movieMatches = append(movieMatches, MovieMatch{
            ID:       match.Vector.ID,
            Title:    match.Vector.Metadata.Title,
            Overview: match.Vector.Metadata.Overview,
            Genres:   match.Vector.Metadata.Genres,
            Year:     match.Vector.Metadata.ReleaseDate,
            Score:    match.Score,
        })
    }

    prompt := createAIPrompt(userPrompt, movieMatches)
    
    aiResponse, err := queryAI(prompt)
    if err != nil {
        return "", fmt.Errorf("failed to get AI recommendations: %v", err)
    }

    aiResponse = strings.TrimPrefix(aiResponse, "```json\n")
    aiResponse = strings.TrimSuffix(aiResponse, "\n```")

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

    if err := json.Unmarshal([]byte(aiResponse), &validationCheck); err != nil {
        return "", fmt.Errorf("invalid JSON structure in AI response: %v", err)
    }

    return aiResponse, nil
}

func createAIPrompt(userPrompt string, movies []MovieMatch) string {
    movieData := make([]map[string]interface{}, len(movies))
    for i, movie := range movies {
        movieData[i] = map[string]interface{}{
            "title":        movie.Title,
            "overview":     movie.Overview,
            "genres":       movie.Genres,
            "release_date": movie.Year,
            "score":        movie.Score,
        }
    }

    promptData := map[string]interface{}{
        "user_query": userPrompt,
        "movies":     movieData,
        "instructions": `You are a movie recommendation system. Analyze these movies based on the user query.
Your response must be a valid JSON object with the exact structure shown below.
For each movie:
1. Research and provide complete movie details including cast, directors, producers, language, and a link to the movie poster
2. Determine if it's relevant to the user's query
3. Provide a clear, very concise explanation of relevance or lack thereof
4. Assign a relevance score from 0.0 to 1.0
5. Extract 3-5 key matching keywords
6. For non-relevant movies, suggest 2-3 alternative movies from similar genres

Your response MUST be in this exact JSON format:
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
            "relevance_explanation": "Clear explanation of why the movie matches or doesn't match the query",
            "keywords": ["keyword1", "keyword2", "keyword3"],
            "relevance_score": 0.95,
            "is_relevant": true,
            "alternative_suggestions": ["Movie 1", "Movie 2"]
        }
    ]
}

Based on the provided movie title and overview, research and include accurate cast, directors, producers, language, and poster URL information. Do not include any text before or after the JSON object. Ensure the response is valid JSON. shorten the length of the responses and their details as much as possible`,
    }

    promptJSON, _ := json.Marshal(promptData)
    return string(promptJSON)
}

func queryAI(prompt string) (string, error) {
    fmt.Println("\n🔄 Starting AI Query...")
    url := "https://openrouter.ai/api/v1/chat/completions"
    
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
        "max_tokens": 4000,
    }

    fmt.Printf("📤 AI Request Body:\n%s\n", prettyPrint(requestBody))

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
    req.Header.Set("Authorization", "Bearer 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")
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

    fmt.Printf("📥 Raw API Response:\n%s\n", prettyPrint(json.RawMessage(body)))

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

    // Clean the response
    content = strings.TrimSpace(content)
    content = strings.TrimPrefix(content, "```json")
    content = strings.TrimPrefix(content, "```")
    content = strings.TrimSuffix(content, "```")
    content = strings.TrimSpace(content)

    fmt.Printf("✅ Successfully received AI response:\n%s\n", prettyPrint(content))

    // Validate JSON
    var jsonCheck interface{}
    if err := json.Unmarshal([]byte(content), &jsonCheck); err != nil {
        fmt.Printf("❌ AI response is not valid JSON: %v\n", err)
        return "", fmt.Errorf("AI response is not valid JSON: %v", err)
    }

    fmt.Println("✅ AI Query Complete")
    return content, nil
}


//sk-8fa662d697834452bdcf7cc5393464cc