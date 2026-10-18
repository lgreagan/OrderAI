@echo off
chcp 65001 >nul
echo 正在创建 OrderAI 项目目录结构...

:: 根目录文件
type nul > main.go
type nul > go.mod
type nul > .env
type nul > build.ps1
type nul > run.ps1

:: config
mkdir config
type nul > config\config.go

:: controllers
mkdir controllers
type nul > controllers\dashboard.go
type nul > controllers\order.go
type nul > controllers\upload.go
type nul > controllers\ai.go
type nul > controllers\product.go

:: services
mkdir services
type nul > services\ai_service.go
type nul > services\deepseek.go
type nul > services\order_service.go
type nul > services\product_service.go
type nul > services\validate_service.go

:: models
mkdir models
type nul > models\order.go
type nul > models\order_item.go
type nul > models\product.go
type nul > models\product_alias.go
type nul > models\ai_result.go

:: repository
mkdir repository
type nul > repository\order_repository.go
type nul > repository\product_repository.go
type nul > repository\ai_repository.go

:: routes
mkdir routes
type nul > routes\routes.go

:: templates
mkdir templates
type nul > templates\layout.html
type nul > templates\index.html
type nul > templates\order_upload.html
type nul > templates\order_preview.html
type nul > templates\order_detail.html

:: web
mkdir web\css
mkdir web\js
type nul > web\css\app.css
type nul > web\js\app.js
type nul > web\js\order.js

:: 数据/日志/上传目录
mkdir uploads
mkdir logs
mkdir sql
type nul > sql\init.sql

echo.
echo ======================================
echo ✅ OrderAI 目录结构创建完成！
echo ======================================
pause
