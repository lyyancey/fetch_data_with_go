# 供应商数据抓取工具 - Go版本

这是Python版本 `fetch_api_data.py` 的Go语言翻译版本。

## 功能特性

- ✅ 从配置文件读取所有配置项
- ✅ 多线程并发抓取数据，提升速度
- ✅ 支持分页抓取大量数据
- ✅ 自动保存为CSV格式（支持Excel打开）
- ✅ 完整的错误处理和进度提示

## 配置文件

在运行之前，请编辑 `config.json` 文件：

```json
{
  "access_token": "your_access_token_here",
  "page_size": 1000,
  "request_delay": 0.5,
  "output_file_prefix": "supplier_data"
}
```

配置项说明：
- `access_token`: 访问令牌（必填），从Chrome开发者工具的Request Headers中复制
- `page_size`: 每页数据量，默认1000条
- `request_delay`: 请求间隔（秒），默认0.5秒
- `output_file_prefix`: 输出文件前缀，默认"supplier_data"

## 运行方法

### 方法1：直接运行
```bash
go run main.go
```

### 方法2：编译后运行
```bash
# 编译
go build -o fetch_data.exe

# 运行
fetch_data.exe
```

### 方法3：使用自定义配置文件
```bash
go run main.go my_config.json
```

## 与Python版本的对比

| 特性 | Python版本 | Go版本 |
|------|-----------|--------|
| 配置文件支持 | ✅ | ✅ |
| 多线程抓取 | ✅ (ThreadPoolExecutor) | ✅ (Goroutines) |
| CSV导出 | ✅ | ✅ |
| 性能 | 快 | 更快 |
| 部署 | 需要Python环境 | 单个可执行文件 |
| 依赖管理 | pip/requirements.txt | go.mod（标准库） |

## 输出文件

程序会自动生成以下格式的CSV文件：
```
supplier_data_20250110_143022.csv
```

文件包含以下字段：
- supplierName (供应商名称)
- unifiedSocialCode (统一社会信用代码)
- updateDate (更新日期)
- companyType (公司类型)
- contactsName (联系人姓名)
- contactsMobilephone (联系人手机)
- 等等...

## 性能优化

Go版本使用以下技术优化性能：

1. **Goroutines并发**: 使用协程池并发抓取多个页面
2. **Channel通信**: 使用channel协调任务分配和结果收集
3. **连接复用**: HTTP客户端自动复用连接
4. **内存优化**: 结构化数据处理，减少内存分配

默认使用5个工作线程，可以在代码中调整 `maxWorkers` 参数。

## 故障排查

### 问题1: "配置文件不存在"
确保 `config.json` 文件与程序在同一目录下。

### 问题2: "access_token 未配置"
在 `config.json` 中填入有效的访问令牌。

### 问题3: "抓取失败"
检查网络连接和访问令牌是否过期。

## 技术栈

- Go 1.24
- 标准库：
  - `encoding/json` - JSON处理
  - `encoding/csv` - CSV导出
  - `net/http` - HTTP客户端
  - `sync` - 并发控制

## 开发者备注

代码结构清晰，主要包含：

1. **Config结构**: 配置文件映射
2. **DataFetcher结构**: 核心抓取器
3. **Payload/Response结构**: API交互数据结构
4. **并发控制**: 使用channel和goroutine实现多线程抓取

欢迎贡献改进！

