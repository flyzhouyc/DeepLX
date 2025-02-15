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
	r.POST("/translate", authMiddleware(cfg), func(c *gin.Context) {
		req := PayloadFree{}
		c.BindJSON(&req)

		sourceLang := req.SourceLang
		targetLang := req.TargetLang
		translateText := req.TransText
		tagHandling := req.TagHandling

		proxyURL := cfg.Proxy

		if tagHandling != "" && tagHandling != "html" && tagHandling != "xml" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    http.StatusBadRequest,
				"message": "Invalid tag_handling value. Allowed values are 'html' and 'xml'.",
			})
			return
		}

		result, err := translate.TranslateByDeepLX(sourceLang, targetLang, translateText, tagHandling, proxyURL, "")
		if err != nil {
			log.Fatalf("Translation failed: %s", err)
		}

		if result.Code == http.StatusOK {
			c.JSON(http.StatusOK, gin.H{
				"code":         http.StatusOK,
				"id":           result.ID,
				"data":         result.Data,
				"alternatives": result.Alternatives,
				"source_lang":  result.SourceLang,
				"target_lang":  result.TargetLang,
				"method":       result.Method,
			})
		} else {
			c.JSON(result.Code, gin.H{
				"code":    result.Code,
				"message": result.Message,
			})

		}
	})

	// Pro API endpoint, Pro Account required
r.POST("/v1/chat/completions", authMiddleware(cfg), func(c *gin.Context) {
    var req ChatCompletionRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request format",
        })
        return
    }

    // 打印接收到的请求
    log.Printf("Received request: %+v", req)

    if len(req.Messages) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "No messages provided",
        })
        return
    }
    lastMessage := req.Messages[len(req.Messages)-1].Content
    log.Printf("Last message: %s", lastMessage)

    sourceLang := ""
    targetLang := "ZH"  // 修改默认目标语言为中文

    if strings.HasPrefix(lastMessage, "Translate to ") {
        parts := strings.SplitN(lastMessage, ":", 2)
        if len(parts) == 2 {
            targetLang = strings.TrimSpace(strings.TrimPrefix(parts[0], "Translate to "))
            lastMessage = strings.TrimSpace(parts[1])
        }
    }

    log.Printf("Translating from %s to %s: %s", sourceLang, targetLang, lastMessage)

    // 调用翻译
    result, err := translate.TranslateByDeepLX(sourceLang, targetLang, lastMessage, "", cfg.Proxy, "")
    if err != nil {
        log.Printf("Translation error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": fmt.Sprintf("Translation failed: %v", err),
        })
        return
    }

    // 打印翻译结果
    log.Printf("Translation result: Code=%d, Message=%s, Data=%s", result.Code, result.Message, result.Data)

    if result.Code != http.StatusOK {
        c.JSON(result.Code, gin.H{
            "error": result.Message,
        })
        return
    }

    // 构造响应
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

    // 打印最终响应
    log.Printf("Final response: %+v", response)

    c.JSON(http.StatusOK, response)
})

	r.Run(fmt.Sprintf("%v:%v", cfg.IP, cfg.Port))
	
}
