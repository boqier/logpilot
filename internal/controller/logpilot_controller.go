/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	logv1 "github.com/boqier/logpilot/api/v1"
	openai "github.com/sashabaranov/go-openai"
)

// LogPilotReconciler reconciles a LogPilot object
type LogPilotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=log.aiops.com,resources=logpilots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=log.aiops.com,resources=logpilots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=log.aiops.com,resources=logpilots/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LogPilot object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *LogPilotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var logpilot logv1.LogPilot
	if err := r.Get(ctx, req.NamespacedName, &logpilot); err != nil {
		log.Error(err, "unable to feach logpolot")
		return ctrl.Result{}, err
	}
	//计算查询日志时间
	currenTime := time.Now().Unix()
	preTimeStamp := logpilot.Status.PreTimeStamp
	fmt.Printf("pretimestamp:%s\n", preTimeStamp)
	var preTime int64
	if preTimeStamp == "" {
		preTime = currenTime - 5
	} else {
		preTime, _ = strconv.ParseInt(preTimeStamp, 10, 64)
	}
	lokiQuery := logpilot.Spec.LokiPromQL
	//纳秒级别时间戳
	endTime := currenTime * 1000000000
	startTime := (preTime - 5) * 1000000000
	fmt.Printf("starttime: %d\n,endtime: %d\n", startTime, endTime)
	if startTime >= endTime {
		log.Info("starttime>=endtime")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	startTimeForUpdate := currenTime
	//调用loki查询日志
	lokiURL := fmt.Sprintf("%s/loki/api/v1/query_range?query=%s&start=%d&end=%d", logpilot.Spec.LokiURL, url.QueryEscape(lokiQuery), startTime, endTime)
	fmt.Println(lokiURL)
	lokiLogs, err := r.queryLoki(lokiURL)
	fmt.Println(lokiLogs)
	if err != nil {
		log.Error(err, "query loki error")
		return ctrl.Result{}, err
	}
	//如果有日志的话，调用LLM接口分析日志，如果有异常，发送飞书告警
	if lokiLogs != "" {
		//TODO 调用LLM接口分析日志
		//TODO 如果有异常，发送飞书告警
		fmt.Printf("send log to llm")
		analysisResult, err := r.analyzeLogsWithLLM(logpilot.Spec.LLMEndpoint, logpilot.Spec.LLMToken, logpilot.Spec.LLMModel, lokiLogs)
		if err != nil {
			log.Error(err, "analyze logs with llm error")
			return ctrl.Result{}, err
		}
		//如果要发送飞书通知就发送
		if analysisResult.Haserror {
			fmt.Printf("send feishu alert")
			err = r.sendFeishuAlert(logpilot.Spec.FeishuWebhook, analysisResult.Analysis)
		}
	}
	//更新状态
	logpilot.Status.PreTimeStamp = fmt.Sprintf("%d", startTimeForUpdate)
	err = r.Status().Update(ctx, &logpilot)
	if err != nil {
		log.Error(err, "update logpilot status error")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *LogPilotReconciler) sendFeishuAlert(webhook, content string) error {
	message := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(messageBytes))
	defer req.Body.Close()
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send feishu alert, status code: %d", resp.StatusCode)
	}
	return nil
}

func (r *LogPilotReconciler) queryLoki(lokiURL string) (string, error) {
	resp, err := http.Get(lokiURL)
	if err != nil {
		return "", fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body error: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	var lokiResponse map[string]interface{}
	if err := json.Unmarshal(body, &lokiResponse); err != nil {
		return "", fmt.Errorf("unmarshal loki response error: %w; body: %s", err, string(body))
	}

	// 检查 Loki status 字段（正常应为 "success"）
	if status, ok := lokiResponse["status"].(string); !ok || status != "success" {
		return "", fmt.Errorf("loki query failed: status=%v, body=%s", lokiResponse["status"], string(body))
	}

	data, ok := lokiResponse["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid loki response data")
	}

	result, ok := data["result"].([]interface{})
	if !ok {
		return "", fmt.Errorf("invalid loki response result structure")
	}

	// ✅ 没有结果不算错误，返回空 body
	if len(result) == 0 {
		fmt.Println("Loki query returned no results (empty result set)")
		return "", nil
	}

	// 正常返回完整 body
	return string(body), nil
}

type LLMAnalysisReSult struct {
	Haserror bool   //是否错误日志
	Analysis string //llm分析结果
}

func (r *LogPilotReconciler) analyzeLogsWithLLM(endpoint, token, model, logs string) (*LLMAnalysisReSult, error) {
	config := openai.DefaultConfig(token)
	config.BaseURL = endpoint
	client := openai.NewClientWithConfig(config)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("你现在是一名运维专家，以下日志是从日志系统中获取的日志，如果遇到严重问题，如外部系统请求失败，系统故障，致命错误，数据库连接异常等严重问题时，给出简短的简易，并且，对于你认为严重的且需要通知运维人员的，在内容中返回[feishu]表示：%s", logs),
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return r.parseLLMResponse(&resp), nil
}
func (r *LogPilotReconciler) parseLLMResponse(resp *openai.ChatCompletionResponse) *LLMAnalysisReSult {
	result := &LLMAnalysisReSult{
		Analysis: resp.Choices[0].Message.Content,
	}
	result.Haserror = false
	//判断结果中是否包含[feishu]
	if strings.Contains(strings.ToLower(result.Analysis), "feishu") {
		result.Haserror = true
	}
	return result

}

// SetupWithManager sets up the controller with the Manager.
func (r *LogPilotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logv1.LogPilot{}).
		Named("logpilot").
		Complete(r)
}
