# Makefile for Meea Icey Docker Operations

.PHONY: help build up down logs clean dev prod test

# 默认目标
help:
	@echo "Available commands:"
	@echo "  build    - Build Docker images"
	@echo "  up       - Start services (development)"
	@echo "  down     - Stop services"
	@echo "  logs     - Show logs"
	@echo "  clean    - Clean up containers and volumes"
	@echo "  dev      - Start development environment"
	@echo "  prod     - Start production environment"
	@echo "  test     - Run tests"

# 构建镜像
build:
	docker-compose build

# 启动开发环境
up:
	docker-compose up -d

# 停止服务
down:
	docker-compose down

# 查看日志
logs:
	docker-compose logs -f

# 清理
clean:
	docker-compose down -v --remove-orphans
	docker system prune -f

# 开发环境
dev:
	docker-compose -f docker-compose.yml -f docker-compose.override.yml up -d

# 生产环境
prod:
	docker-compose -f docker-compose.prod.yml up -d

# 运行测试
test:
	docker-compose run --rm app go test ./...

# 进入容器
shell:
	docker-compose exec app sh

# 重启服务
restart:
	docker-compose restart

# 查看状态
status:
	docker-compose ps

# 构建并推送镜像
build-push:
	docker-compose build
	docker tag meea-icey-app:latest your-registry/meea-icey:latest
	docker push your-registry/meea-icey:latest 