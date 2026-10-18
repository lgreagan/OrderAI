package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"OrderAI/models"
)

type DeepSeekService struct {
	APIKey  string
	BaseURL string
	Model   string
}

type DeepSeekAPIError struct {
	StatusCode int
	Response   string
}

func (e *DeepSeekAPIError) Error() string {
	if e.StatusCode == http.StatusPaymentRequired {
		return "DeepSeek 账户余额不足，请为当前 API Key 所属账户充值后重试"
	}

	return fmt.Sprintf("DeepSeek API 返回 HTTP %d: %s", e.StatusCode, e.Response)
}

func NewDeepSeekService(
	apiKey string,
	baseURL string,
	model string,
) *DeepSeekService {

	return &DeepSeekService{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
	}
}

type deepSeekRequest struct {
	Model          string            `json:"model"`
	Messages       []deepSeekMessage `json:"messages"`
	ResponseFormat map[string]string `json:"response_format"`
	Thinking       *deepSeekThinking `json:"thinking,omitempty"`
}

type deepSeekThinking struct {
	Type string `json:"type"`
}

type deepSeekMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type deepSeekContent struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *deepSeekImageURL `json:"image_url,omitempty"`
}

type deepSeekImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *DeepSeekService) RecognizeOrder(
	imagePath string,
) (*models.AIOrderResult, error) {

	if s.APIKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY 未配置")
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取订单图片失败: %w", err)
	}

	mediaType := detectImageType(imagePath)

	base64Image := base64.StdEncoding.EncodeToString(imageData)

	dataURL := fmt.Sprintf(
		"data:%s;base64,%s",
		mediaType,
		base64Image,
	)

	prompt := `
你是一个专业的国际贸易订单识别系统。

请仔细读取这张客户订单图片，并提取其中的订单信息。

你的任务不是简单 OCR，而是理解订单中的表格、产品描述、规格、包装方式和数量关系。

必须遵守以下规则：

1. 识别订单号。
2. 识别国家。
3. 识别运输方式，例如 SEA、AIR、EXPRESS。
4. 识别客户名称，如果图片中没有则为空。
5. 逐行识别所有产品。
6. 保留客户原始产品描述。
7. 识别产品规格。
8. 识别数量。
9. 识别单位，例如 PCS、Pack、Bag、Set、Box。
10. 如果存在包装关系，识别 pack_size。
11. 如果客户同时给出了包装数量和实际数量，必须分别保存。
12. 不允许自行创造 SKU。
13. 不允许猜测图片中没有出现的信息。
14. 如果某个字段无法确定，使用空字符串或 0。
15. declared_quantity 必须尽量保留客户图片中的原始数量表达。
16. actual_quantity 表示根据包装关系计算出来的实际产品数量。
17. 如果图片明确给出了实际数量，优先使用图片中的实际数量。
18. 如果 quantity=80，pack_size=2，则 actual_quantity=160。
19. 如果 quantity=321，unit=Bag，pack_size=2，则 actual_quantity=642。
20. 如果数量存在明显疑点，在 notes 中说明。
21. 最终只能输出 JSON，不要输出 Markdown，不要输出解释。

返回以下 JSON 结构：

{
  "order_number": "",
  "country": "",
  "shipping_method": "",
  "customer": "",
  "items": [
    {
      "line_number": 1,
      "product_name": "",
      "description": "",
      "specification": "",
      "quantity": 0,
      "unit": "",
      "pack_size": 1,
      "actual_quantity": 0,
      "declared_quantity": ""
    }
  ],
  "notes": []
}
`

	content := []deepSeekContent{
		{
			Type: "text",
			Text: prompt,
		},
		{
			Type: "image_url",
			ImageURL: &deepSeekImageURL{
				URL:    dataURL,
				Detail: "original",
			},
		},
	}

	reqBody := deepSeekRequest{
		Model: s.Model,
		Messages: []deepSeekMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
		ResponseFormat: map[string]string{
			"type": "json_object",
		},
		Thinking: &deepSeekThinking{
			Type: "disabled",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("生成 DeepSeek 请求失败: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		s.BaseURL+"/chat/completions",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 DeepSeek 请求失败: %w", err)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+s.APIKey,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 DeepSeek 失败: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 DeepSeek 响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &DeepSeekAPIError{
			StatusCode: resp.StatusCode,
			Response:   string(body),
		}
	}

	var result deepSeekResponse

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf(
			"解析 DeepSeek 响应失败: %w\n%s",
			err,
			string(body),
		)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("DeepSeek 没有返回识别结果")
	}

	contentText := result.Choices[0].Message.Content

	var order models.AIOrderResult

	if err := json.Unmarshal(
		[]byte(contentText),
		&order,
	); err != nil {

		return nil, fmt.Errorf(
			"解析订单 JSON 失败: %w\nAI返回内容：%s",
			err,
			contentText,
		)
	}

	return &order, nil
}

func detectImageType(path string) string {

	ext := strings.ToLower(
		filepath.Ext(path),
	)

	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"

	case ".png":
		return "image/png"

	case ".gif":
		return "image/gif"

	case ".webp":
		return "image/webp"

	default:
		return "image/jpeg"
	}
}
