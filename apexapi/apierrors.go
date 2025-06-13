package apexapi

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyURL            = errors.New("提供的URL为空")
	ErrEmptyAPIToken       = errors.New("未填入API Token")
	ErrWrongAPIToken       = errors.New("API Token错误")
	ErrRequestCreateFailed = errors.New("构造请求失败")
	ErrRequestFailed       = errors.New("请求失败")
	ErrInvalidJSON         = errors.New("JSON解析错误")
	Err403Forbidden        = errors.New("API密钥出错[403]")
	ErrNoPlayerFound       = errors.New("未找到该玩家，请检查名称后重试")
	ErrReadResponseFailed  = errors.New("读取响应内容失败")
	ErrAPIReturnedError    = func(msg string) error {
		return errors.New("API返回错误: " + msg)
	}
	ErrUnexpectedStatusCode = func(code int) error {
		return errors.New("收到意外状态码: " + fmt.Sprint(code))
	}
	ErrStatusCode = func(code int) error {
		switch code {
		case 400:
			return ErrAPIReturnedError("API请求出错，请稍后再试吧o.0")
		case 403:
			return Err403Forbidden
		case 404:
			return ErrNoPlayerFound
		case 405:
			return ErrAPIReturnedError("外部API错误，请联系管理员修复>w<")
		case 410:
			return ErrAPIReturnedError("未知平台，请联系管理员修复>w<")
		case 429:
			return ErrAPIReturnedError("API速率限制，请稍后再试吧X_X")
		case 503:
			return ErrAPIReturnedError("似乎API服务器不可用，请稍后再试吧")
		default:
			return fmt.Errorf("%w: 状态码%d", ErrUnexpectedStatusCode(code), code)
		}
	}
)
