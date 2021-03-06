package gateway

import (
	"fmt"
	"net/http"

	log "github.com/eolinker/goku-api-gateway/goku-log"
	"github.com/eolinker/goku-api-gateway/goku-node/common"
	plugin_executor "github.com/eolinker/goku-api-gateway/node/gateway/plugin-executor"
	"github.com/eolinker/goku-api-gateway/node/router"
	access_field "github.com/eolinker/goku-api-gateway/server/access-field"
)

//Strategy strategy
type Strategy struct {
	ID     string
	Name   string
	Enable bool

	apiRouter router.APIRouter

	accessPlugin       []plugin_executor.Executor
	globalAccessPlugin []plugin_executor.Executor

	authPlugin map[string]plugin_executor.Executor

	isNeedAuth bool
}

//Router router
func (r *Strategy) Router(w http.ResponseWriter, req *http.Request, ctx *common.Context) {
	if !r.Enable {

		go log.Info("[ERROR]StrategyID is out of service!")

		ctx.SetStatus(500, "500")

		ctx.SetBody([]byte("[ERROR]StrategyID is out of service!"))
		return
	}

	ctx.SetStrategyName(r.Name)
	ctx.SetStrategyId(r.ID)
	ctx.LogFields[access_field.Strategy] = fmt.Sprintf("\"%s %s\"", r.ID, r.Name)

	if r.isNeedAuth {
		// 需要校验
		if !r.auth(ctx) {
			// 校验失败
			ctx.SetStatus(403, "403")
			ctx.SetBody([]byte("[ERROR]Illegal authorization type!"))
			return
		}
	}

	r.apiRouter.ServeHTTP(w, req, ctx)
}

func (r *Strategy) auth(ctx *common.Context) bool {
	requestID := ctx.RequestId()
	authType := ctx.Request().GetHeader("Authorization-Type")
	authPlugin, has := r.authPlugin[authType]
	if !has {
		log.Warn(requestID, " Illegal authorization type:", authType)
		return false
	}

	isContinue, err := authPlugin.Execute(ctx)
	if isContinue == false {
		pluginName := authNames[authType]
		// 校验失败
		if err != nil {
			log.Warn(requestID, " access auth:[", pluginName, "] error:", err)
		}
		log.Info(requestID, " auth [", pluginName, "] refuse")

		return false
	}
	log.Debug(requestID, " auth [", authType, "] pass")
	return true
}
func (r *Strategy) accessFlow(ctx *common.Context) {
	for _, handler := range r.accessPlugin {

		flag, _ := handler.Execute(ctx)

		if flag == false && handler.IsStop() {

			return
		}
	}
}

func (r *Strategy) accessGlobalFlow(ctx *common.Context) {
	// 全局插件不中断
	for _, handler := range r.globalAccessPlugin {
		_, _ = handler.Execute(ctx)

	}
}

//HandlerAPINotFound 当接口不存在时调用
func (r *Strategy) HandlerAPINotFound(ctx *common.Context) {
	// 未匹配到api
	// 执行策略access 插件
	r.accessFlow(ctx)
	// 执行全局access 插件
	r.accessGlobalFlow(ctx)

	log.Info(ctx.RequestId(), " URL dose not exist!")
	ctx.SetStatus(404, "404")
	ctx.SetBody([]byte("[ERROR]URL dose not exist!"))
}
