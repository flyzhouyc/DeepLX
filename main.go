/*
 * @Author: Vincent Yang
 * @Date: 2023-07-01 21:45:34
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2024-11-01 13:04:50
 * @FilePath: /DeepLX/main.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package main

import (
	//"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	

	translate "github.com/OwO-Network/DeepLX/translate"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func authMiddleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Token != "" {
			providedTokenInQuery := c.Query("token")
			providedTokenInHeader := c.GetHeader("Authorization")

			// Compatability with the Bearer token format
			if providedTokenInHeader != "" {
				parts := strings.Split(providedTokenInHeader, " ")
				if len(parts) == 2 {
					if parts[0] == "Bearer" || parts[0] == "DeepL-Auth-Key" {
						providedTokenInHeader = parts[1]
					} else {
						providedTokenInHeader = ""
					}
				} else {
					providedTokenInHeader = ""
				}
			}

			if providedTokenInHeader != cfg.Token && providedTokenInQuery != cfg.Token {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    http.StatusUnauthorized,
					"message": "Invalid access token",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

type PayloadFree struct {
	TransText   string `json:"text"`
	SourceLang  string `json:"source_lang"`
	TargetLang  string `json:"target_lang"`
	TagHandling string `json:"tag_handling"`
}

type PayloadAPI struct {
	Text        []string `json:"text"`
	TargetLang  string   `json:"target_lang"`
	SourceLang  string   `json:"source_lang"`
	TagHandling string   `json:"tag_handling"`
}
type ChatCompletionRequest struct {
    Messages []struct {
        Role    string `json:"role"`
        Content string `json:"content"`
    } `json:"messages"`
    Model string `json:"model"`
    Stream bool   `json:"stream"`
}

type ChatCompletionResponse struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Index        int `json:"index"`
        Message     struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"message"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
}
type ChatCompletionChunk struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []ChunkChoice `json:"choices"`
}

type ChunkChoice struct {
    Index        int         `json:"index"`
    Delta        DeltaStruct `json:"delta"`
    FinishReason *string     `json:"finish_reason"`
}

type DeltaStruct struct {
    Content string `json:"content"`
    Role    string `json:"role,omitempty"`
}
func writeSSE(c *gin.Context, data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    _, err = c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
    if err != nil {
        return err
    }
    c.Writer.Flush()
    return nil
}

func main() {
	cfg := initConfig()

	fmt.Printf("DeepL X has been successfully launched! Listening on %v:%v\n", cfg.IP, cfg.Port)
	fmt.Println("Developed by sjlleo <i@leo.moe> and missuo <me@missuo.me>.")

	// Set Proxy
	proxyURL := os.Getenv("PROXY")
	if proxyURL == "" {
		proxyURL = cfg.Proxy
	}
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			log.Fatalf("Failed to parse proxy URL: %v", err)
		}
		http.DefaultTransport = &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
	}

	if cfg.Token != "" {
		fmt.Println("Access token is set.")
	}

	// Setting the application to release mode
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())

	// Defining the root endpoint which returns the project details
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "DeepL Free API, Developed by sjlleo and missuo. Go to /translate with POST. http://github.com/OwO-Network/DeepLX",
		})
	})

	// Free API endpoint, No Pro Account required
	r.POST("/v1/chat/completions", authMiddleware(cfg), func(c *gin.Context) {
    var req ChatCompletionRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request format",
        })
        return
    }

    if len(req.Messages) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "No messages provided",
        })
        return
    }
    
    lastMessage := req.Messages[len(req.Messages)-1].Content
    sourceLang := ""
    targetLang := ""

    // 根据model名称决定翻译方向
    switch req.Model {
    case "deepl-zh-en":
        sourceLang = "ZH"
        targetLang = "EN"
    case "deepl-en-zh":
        sourceLang = "EN"
        targetLang = "ZH"
    case "deepl-auto-zh":
        sourceLang = ""
        targetLang = "ZH"
    case "deepl-auto-en":
        sourceLang = ""
        targetLang = "EN"
    default:
        sourceLang = ""
        targetLang = "ZH"
    }

    if strings.HasPrefix(lastMessage, "Translate to ") {
        parts := strings.SplitN(lastMessage, ":", 2)
        if len(parts) == 2 {
            targetLang = strings.TrimSpace(strings.TrimPrefix(parts[0], "Translate to "))
            lastMessage = strings.TrimSpace(parts[1])
        }
    }

    result, err := translate.TranslateByDeepLX(sourceLang, targetLang, lastMessage, "", cfg.Proxy, cfg.DlSession)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": fmt.Sprintf("Translation failed: %v", err),
        })
        return
    }

    if result.Code != http.StatusOK {
        c.JSON(result.Code, gin.H{
            "error": result.Message,
        })
        return
    }

    // 判断是否为流式请求
    if req.Stream {
        // 流式响应
        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("Transfer-Encoding", "chunked")

        // 发送角色信息
  chunk := ChatCompletionChunk{
    ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
    Object:  "chat.completion.chunk",
    Created: time.Now().Unix(),
    Model:   req.Model,
    Choices: []ChunkChoice{{
        Index: 0,
        Delta: DeltaStruct{
            Role: "assistant",
        },
        FinishReason: nil,
    }},
}


        if err := writeSSE(c, chunk); err != nil {
            log.Printf("Error writing SSE: %v", err)
            return
        }

        // 发送翻译内容
        chunk.Choices[0].Delta.Role = ""
        chunk.Choices[0].Delta.Content = result.Data
        if err := writeSSE(c, chunk); err != nil {
            log.Printf("Error writing SSE: %v", err)
            return
        }

        // 发送完成标记
        finishReason := "stop"
        chunk.Choices[0].Delta.Content = ""
        chunk.Choices[0].FinishReason = &finishReason
        if err := writeSSE(c, chunk); err != nil {
            log.Printf("Error writing SSE: %v", err)
            return
        }

        if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
            log.Printf("Error writing final SSE: %v", err)
        }
        c.Writer.Flush()
    } else {
        // 非流式响应
        response := ChatCompletionResponse{
            ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
            Object:  "chat.completion",
            Created: time.Now().Unix(),
            Model:   req.Model,
            Choices: []struct {
                Index        int `json:"index"`
                Message     struct {
                    Role    string `json:"role"`
                    Content string `json:"content"`
                } `json:"message"`
                FinishReason string `json:"finish_reason"`
            }{
                {
                    Index: 0,
                    Message: struct {
                        Role    string `json:"role"`
                        Content string `json:"content"`
                    }{
                        Role:    "assistant",
                        Content: result.Data,
                    },
                    FinishReason: "stop",
                },
            },
            Usage: struct {
                PromptTokens     int `json:"prompt_tokens"`
                CompletionTokens int `json:"completion_tokens"`
                TotalTokens      int `json:"total_tokens"`
            }{
                PromptTokens:     len(lastMessage),
                CompletionTokens: len(result.Data),
                TotalTokens:      len(lastMessage) + len(result.Data),
            },
        }

        c.JSON(http.StatusOK, response)
    }
})



	r.Run(fmt.Sprintf("%v:%v", cfg.IP, cfg.Port))
}
