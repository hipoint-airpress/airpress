package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type EmailHandler struct {
	EmailService service.EmailService
}

func NewEmailHandler(emailService service.EmailService) *EmailHandler {
	return &EmailHandler{
		EmailService: emailService,
	}
}

// Test godoc
// @Summary      发送测试邮件
// @Description  使用指定参数发送一封测试邮件
// @Tags         Admin.Email
// @Accept       json
// @Produce      json
// @Param        testEmail  body     param.TestEmail  true  "测试邮件参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/mails/test [post]
func (e *EmailHandler) Test(ctx *gin.Context) (interface{}, error) {
	p := &param.TestEmail{}
	if err := ctx.ShouldBindJSON(p); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("param error ")
	}
	return nil, e.EmailService.SendTextEmail(ctx, p.To, p.Subject, p.Content)
}
