package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Config 配置文件结构
type Config struct {
	AccessToken      string  `json:"access_token"`
	PageSize         int     `json:"page_size"`
	RequestDelay     float64 `json:"request_delay"`
	OutputFilePrefix string  `json:"output_file_prefix"`
	MaxWorkers       int     `json:"max_workers"`
	BaseURL          string  `json:"base_url"`
}

// DataFetcher 数据抓取器
type DataFetcher struct {
	config           Config
	accessToken      string
	pageSize         int
	requestDelay     time.Duration
	outputFilePrefix string
	baseURL          string
	csvHeaders       []string
	client           *http.Client
	maxWorkers       int
}

// PayloadBlock 请求体中的块结构
type PayloadBlock struct {
	Meta struct {
		Desc    string        `json:"desc,omitempty"`
		Attr    interface{}   `json:"attr,omitempty"`
		Columns []interface{} `json:"columns"`
	} `json:"meta"`
	Rows [][]interface{} `json:"rows"`
	Attr interface{}     `json:"attr,omitempty"`
}

// Payload 请求体结构
type Payload struct {
	ServiceName string                  `json:"serviceName"`
	MethodName  string                  `json:"methodName"`
	Context     map[string]interface{}  `json:"__context__"`
	User        map[string]interface{}  `json:"__user__"`
	Version     string                  `json:"__version__"`
	Sys         map[string]interface{}  `json:"__sys__"`
	Blocks      map[string]PayloadBlock `json:"__blocks__"`
}

// Response 响应结构
type Response struct {
	Blocks map[string]struct {
		Rows [][]interface{} `json:"rows"`
		Attr struct {
			Count int `json:"count"`
		} `json:"attr"`
	} `json:"__blocks__"`
}

// NewDataFetcher 创建数据抓取器实例
func NewDataFetcher(configFile string) (*DataFetcher, error) {
	// 加载配置文件
	config, err := loadConfig(configFile)
	if err != nil {
		return nil, err
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	df := &DataFetcher{
		config:           config,
		accessToken:      config.AccessToken,
		pageSize:         config.PageSize,
		requestDelay:     time.Duration(config.RequestDelay * float64(time.Second)),
		outputFilePrefix: config.OutputFilePrefix,
		baseURL:          config.BaseURL,
		maxWorkers:       config.MaxWorkers,
		csvHeaders: []string{
			"supplierName", "unifiedSocialCode", "updateDate",
			"domesticForeignRelation", "companyType", "licenceEndDate",
			"updateUserName", "updateUser", "institutionType",
			"createUserName", "supplierCode", "contactsName",
			"contactsMobilephone", "licenceFromDate", "addressDetail",
			"offlineSupplier", "contactsMail", "createUser",
			"internalCode", "contactsTelephone", "createDate",
		},
		client: client,
	}

	// 设置默认值
	if df.pageSize == 0 {
		df.pageSize = 1000
	}
	if df.requestDelay == 0 {
		df.requestDelay = 500 * time.Millisecond
	}
	if df.outputFilePrefix == "" {
		df.outputFilePrefix = "supplier_data"
	}
	if df.maxWorkers == 0 {
		df.maxWorkers = 5
	}
	if df.baseURL == "" {
		df.baseURL = "https://one.cnncecp.com/cnnc-ps-api/"
	}

	return df, nil
}

// loadConfig 加载配置文件
func loadConfig(configFile string) (Config, error) {
	var config Config

	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(configFile) {
		dir, _ := os.Getwd()
		configFile = filepath.Join(dir, configFile)
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return config, fmt.Errorf("配置文件不存在: %s", configFile)
	}

	// 解析JSON
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("配置文件格式错误: %v", err)
	}

	fmt.Printf("✓ 成功加载配置文件: %s\n", configFile)
	return config, nil
}

// buildHeaders 构建请求头
func (df *DataFetcher) buildHeaders() map[string]string {
	return map[string]string{
		"ACCESS-No":          df.accessToken,
		"Accept":             "application/json, text/plain, */*",
		"Accept-Encoding":    "gzip, deflate, br, zstd",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Access-Token":       df.accessToken,
		"Connection":         "keep-alive",
		"Content-Type":       "application/json;charset=UTF-8",
		"Cookie":             fmt.Sprintf("_tea_utm_cache_10000007=undefined; token=%s", df.accessToken),
		"DNT":                "1",
		"Host":               "one.cnncecp.com",
		"Mk-Request":         "1",
		"Origin":             "https://one.cnncecp.com",
		"Referer":            "https://one.cnncecp.com/cnnc-pm-web/",
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		"menuId":             "PCPSAM26",
		"sec-ch-ua":          `"Google Chrome";v="141", "Not?A_Brand";v="8", "Chromium";v="141"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sso_token":          df.accessToken,
	}
}

// buildPayload 构建请求体
func (df *DataFetcher) buildPayload() Payload {
	return Payload{
		ServiceName: "PSRM01",
		MethodName:  "querySupCm",
		Context:     make(map[string]interface{}),
		User:        make(map[string]interface{}),
		Version:     "2.0",
		Sys: map[string]interface{}{
			"name":      "",
			"descName":  "",
			"msg":       "",
			"msgKey":    "",
			"detailMsg": "",
			"status":    0,
			"traceId":   "",
		},
		Blocks: map[string]PayloadBlock{
			"result": {
				Meta: struct {
					Desc    string        `json:"desc,omitempty"`
					Attr    interface{}   `json:"attr,omitempty"`
					Columns []interface{} `json:"columns"`
				}{
					Columns: []interface{}{},
				},
				Rows: [][]interface{}{{}},
				Attr: map[string]interface{}{
					"limit":     10,
					"offset":    10,
					"showCount": "true",
				},
			},
			"inqu_status": {
				Meta: struct {
					Desc    string        `json:"desc,omitempty"`
					Attr    interface{}   `json:"attr,omitempty"`
					Columns []interface{} `json:"columns"`
				}{
					Desc: "",
					Attr: map[string]interface{}{},
					Columns: []interface{}{
						map[string]interface{}{"pos": 0, "name": "supplierCode"},
						map[string]interface{}{"pos": 1, "name": "supplierName"},
						map[string]interface{}{"pos": 2, "name": "companyType"},
						map[string]interface{}{"pos": 3, "name": "offlineSupplier"},
						map[string]interface{}{"pos": 4, "name": "unifiedSocialCode"},
						map[string]interface{}{"pos": 5, "name": "aliveFlag"},
					},
				},
				Rows: [][]interface{}{{"", "", "", "", "", "1"}},
				Attr: map[string]interface{}{},
			},
		},
	}
}

// fetchData 发送POST请求获取数据
func (df *DataFetcher) fetchData(ctx context.Context, payload Payload) (*Response, error) {
	// 序列化payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", df.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	headers := df.buildHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := df.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d, 响应内容: %s", resp.StatusCode, string(body))
	}

	// 解析JSON
	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		// 尝试打印前100个字符以供调试
		preview := string(body)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return nil, fmt.Errorf("JSON解析失败: %v, 响应内容: %s", err, preview)
	}

	return &response, nil
}

// PageResult 页面抓取结果
type PageResult struct {
	PageNum int
	Rows    [][]interface{}
	Err     error
}

// FetchAllDataMultithread 多线程分页抓取数据
func (df *DataFetcher) FetchAllDataMultithread(ctx context.Context, basePayload Payload, csvFilename string) (int, error) {
	if csvFilename == "" {
		timestamp := time.Now().Format("20060102_150405")
		csvFilename = fmt.Sprintf("%s_%s.csv", df.outputFilePrefix, timestamp)
	}

	fmt.Printf("\n开始多线程抓取数据...\n")
	fmt.Printf("每页大小: %d 条\n", df.pageSize)
	fmt.Printf("最大线程数: %d\n", df.maxWorkers)
	fmt.Printf("输出文件: %s\n", csvFilename)
	fmt.Println("======================================================================")

	// 先请求第一页，获取总数
	payload := basePayload
	resultAttr := payload.Blocks["result"].Attr.(map[string]interface{})
	resultAttr["limit"] = df.pageSize
	resultAttr["offset"] = 0

	block := payload.Blocks["result"]
	block.Attr = resultAttr
	payload.Blocks["result"] = block

	response, err := df.fetchData(ctx, payload)
	if err != nil {
		return 0, fmt.Errorf("首次请求失败: %v", err)
	}

	resultBlock, ok := response.Blocks["result"]
	if !ok {
		return 0, fmt.Errorf("响应数据格式异常")
	}

	totalCount := resultBlock.Attr.Count
	if totalCount == 0 {
		fmt.Println("❌ 未能获取总数据量")
		return 0, nil
	}

	totalPages := (totalCount + df.pageSize - 1) / df.pageSize
	fmt.Printf("✓ 从服务器获取到总数据量: %d 条\n", totalCount)
	fmt.Printf("✓ 预计总页数: %d 页\n", totalPages)
	fmt.Println("======================================================================")

	// 创建CSV文件
	file, err := os.Create(csvFilename)
	if err != nil {
		return 0, fmt.Errorf("创建CSV文件失败: %v", err)
	}
	defer file.Close()

	// 写入UTF-8 BOM
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	writer.Write(df.csvHeaders)

	// 创建任务通道和结果通道
	tasks := make(chan struct {
		pageNum int
		offset  int
	}, totalPages)
	results := make(chan PageResult, df.maxWorkers) // 缓冲结果，避免阻塞worker

	// 启动结果处理协程（写入CSV）
	var writeWg sync.WaitGroup
	writeWg.Add(1)
	totalRows := 0
	go func() {
		defer writeWg.Done()
		for result := range results {
			if result.Err != nil {
				fmt.Printf("❌ 第%d页抓取失败: %v\n", result.PageNum, result.Err)
				continue
			}
			if result.Rows != nil {
				for _, row := range result.Rows {
					strRow := make([]string, len(row))
					for i, cell := range row {
						if cell == nil {
							strRow[i] = "\t"
						} else {
							strRow[i] = fmt.Sprintf("\t%v", cell)
						}
					}
					writer.Write(strRow)
					totalRows++
				}
				writer.Flush() // 及时刷新
			}
		}
	}()

	// 填充任务
	go func() {
		for page := 0; page < totalPages; page++ {
			select {
			case <-ctx.Done():
				close(tasks)
				return
			case tasks <- struct {
				pageNum int
				offset  int
			}{
				pageNum: page + 1,
				offset:  page * df.pageSize,
			}:
			}
		}
		close(tasks)
	}()

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < df.maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				// 检查上下文是否取消
				select {
				case <-ctx.Done():
					return
				default:
				}

				// 复制payload
				payload := basePayload
				resultAttr := payload.Blocks["result"].Attr.(map[string]interface{})
				resultAttr["limit"] = df.pageSize
				resultAttr["offset"] = task.offset

				block := payload.Blocks["result"]
				block.Attr = resultAttr
				payload.Blocks["result"] = block

				// 抓取数据
				response, err := df.fetchData(ctx, payload)
				if err != nil {
					results <- PageResult{PageNum: task.pageNum, Rows: nil, Err: err}
					continue
				}

				resultBlock, ok := response.Blocks["result"]
				if !ok {
					results <- PageResult{PageNum: task.pageNum, Rows: [][]interface{}{}, Err: nil}
					continue
				}

				rows := resultBlock.Rows
				fmt.Printf("✓ 第%d页(offset=%d) 获取%d条数据\n", task.pageNum, task.offset, len(rows))
				results <- PageResult{PageNum: task.pageNum, Rows: rows, Err: nil}

				// 延迟
				time.Sleep(df.requestDelay)
			}
		}()
	}

	// 等待所有工作协程完成
	wg.Wait()
	close(results)

	// 等待写入完成
	writeWg.Wait()

	fmt.Println("======================================================================")
	if ctx.Err() != nil {
		fmt.Printf("\n⚠️ 任务被中断！共保存 %d 条数据\n", totalRows)
	} else {
		fmt.Printf("\n✅ 多线程抓取完成！共保存 %d 条数据\n", totalRows)
	}
	return totalRows, nil
}

func main() {
	// 设置信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️ 接收到中断信号，正在停止...")
		cancel()
	}()

	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	fmt.Println("======================================================================")
	fmt.Println("供应商数据抓取工具 - Go版本")
	fmt.Println("======================================================================")

	// 创建数据抓取器实例
	fetcher, err := NewDataFetcher(configFile)
	if err != nil {
		fmt.Printf("\n初始化失败: %v\n", err)
		return
	}

	// 验证Token是否配置
	if fetcher.accessToken == "" {
		fmt.Println("❌ 请在配置文件中设置 access_token")
		fmt.Println("💡 提示：从Chrome控制台的Request Headers中复制 Access-Token 的值")
		return
	}

	// 使用固定的请求体模板
	basePayload := fetcher.buildPayload()

	fmt.Printf("目标URL: %s\n", fetcher.baseURL)
	fmt.Printf("服务名称: %s\n", basePayload.ServiceName)
	fmt.Printf("方法名称: %s\n", basePayload.MethodName)
	fmt.Printf("每页大小: %d 条\n", fetcher.pageSize)
	fmt.Printf("请求间隔: %.1f 秒\n", fetcher.requestDelay.Seconds())
	if len(fetcher.accessToken) > 20 {
		fmt.Printf("Token: %s...\n", fetcher.accessToken[:20])
	} else {
		fmt.Printf("Token: %s\n", fetcher.accessToken)
	}

	// 多线程抓取并保存数据
	totalRows, err := fetcher.FetchAllDataMultithread(ctx, basePayload, "")
	if err != nil {
		fmt.Printf("\n❌ 抓取失败: %v\n", err)
		return
	}

	if totalRows > 0 {
		fmt.Println("\n✅ 所有任务完成！")
		fmt.Printf("   数据总量: %d 条\n", totalRows)
		fmt.Println("\n💡 提示: 可以用Excel或其他工具打开CSV文件查看数据")
	} else {
		fmt.Println("\n❌ 未能获取任何数据")
	}
}
