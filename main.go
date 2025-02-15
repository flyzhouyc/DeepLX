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
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	//"encoding/json"

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
    targetLang := "ZH"

    if strings.HasPrefix(lastMessage, "Translate to ") {
        parts := strings.SplitN(lastMessage, ":", 2)
        if len(parts) == 2 {
            targetLang = strings.TrimSpace(strings.TrimPrefix(parts[0], "Translate to "))
            lastMessage = strings.TrimSpace(parts[1])
        }
    }

    // 调用翻译
    result, err := translate.TranslateByDeepLX(sourceLang, targetLang, lastMessage, "", cfg.Proxy, "")
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

    // 设置响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Transfer-Encoding", "chunked")

    // 创建基本响应结构
    baseResponse := struct {
        ID      string `json:"id"`
        Object  string `json:"object"`
        Created int64  `json:"created"`
        Model   string `json:"model"`
        Choices []struct {
            Index        int    `json:"index"`
            Delta       struct {
                Content string `json:"content"`
                Role    string `json:"role,omitempty"`
            } `json:"delta"`
            FinishReason *string `json:"finish_reason"`
        } `json:"choices"`
    }{
        ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
        Object:  "chat.completion.chunk",
        Created: time.Now().Unix(),
        Model:   req.Model,
        Choices: []struct {
            Index        int    `json:"index"`
            Delta       struct {
                Content string `json:"content"`
                Role    string `json:"role,omitempty"`
            } `json:"delta"`
            FinishReason *string `json:"finish_reason"`
        }{{
            Index: 0,
            Delta: struct {
                Content string `json:"content"`
                Role    string `json:"role,omitempty"`
            }{
                Role: "assistant",
            },
        }},
    }

    // 发送角色信息
    jsonData, _ := json.Marshal(baseResponse)
    c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
    c.Writer.Flush()

    // 发送翻译内容
    baseResponse.Choices[0].Delta.Role = ""
    baseResponse.Choices[0].Delta.Content = result.Data
    jsonData, _ = json.Marshal(baseResponse)
    c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
    c.Writer.Flush()

    // 发送完成标记
    finishReason := "stop"
    baseResponse.Choices[0].Delta.Content = ""
    baseResponse.Choices[0].FinishReason = &finishReason
    jsonData, _ = json.Marshal(baseResponse)
    c.Writer.Write([]byte("data: " + string(jsonData) + "\n\n"))
    c.Writer.Write([]byte("data: [DONE]\n\n"))
    c.Writer.Flush()
})
	r.Run(fmt.Sprintf("%v:%v", cfg.IP, cfg.Port))
}
