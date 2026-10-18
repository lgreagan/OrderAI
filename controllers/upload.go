package controllers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
)

func UploadPage(c echo.Context) error {
	return c.Render(http.StatusOK, "order_upload.html", map[string]interface{}{
		"title": "订单图片识别",
	})
}

func UploadOrderImage(c echo.Context) error {
	file, err := c.FormFile("order_image")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "请选择订单图片",
		})
	}

	// 检查扩展名
	ext := filepath.Ext(file.Filename)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "只支持 JPG、JPEG、PNG、WEBP 图片",
		})
	}

	// 创建上传目录
	uploadDir := "uploads"

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "无法创建上传目录",
		})
	}

	// 生成唯一文件名
	filename := fmt.Sprintf(
		"%s_%d%s",
		"order",
		time.Now().UnixNano(),
		ext,
	)

	savePath := filepath.Join(uploadDir, filename)

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "读取图片失败",
		})
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "保存图片失败",
		})
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(savePath)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "保存图片失败",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "图片上传成功",
		"filename": filename,
		"url":      "/uploads/" + filename,
	})
}

// UploadSKUImage 保存 SKU（产品）图片，返回可直接作为 <img src> 使用的地址。
func UploadSKUImage(c echo.Context) error {
	file, err := c.FormFile("image")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "请选择产品图片",
		})
	}

	// 检查扩展名
	ext := filepath.Ext(file.Filename)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "只支持 JPG、JPEG、PNG、WEBP 图片",
		})
	}

	// 创建上传目录
	uploadDir := "uploads"

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "无法创建上传目录",
		})
	}

	// 生成唯一文件名
	filename := fmt.Sprintf(
		"%s_%d%s",
		"sku",
		time.Now().UnixNano(),
		ext,
	)

	savePath := filepath.Join(uploadDir, filename)

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "读取图片失败",
		})
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "保存图片失败",
		})
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(savePath)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "保存图片失败",
		})
	}

	image := "/uploads/" + filename

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "图片上传成功",
		"data": map[string]interface{}{
			"image": image,
			"url":   image,
		},
	})
}
