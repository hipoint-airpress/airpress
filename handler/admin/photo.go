package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type PhotoHandler struct {
	PhotoService service.PhotoService
}

func NewPhotoHandler(photoService service.PhotoService) *PhotoHandler {
	return &PhotoHandler{
		PhotoService: photoService,
	}
}

// ListPhoto godoc
// @Summary      查询最新照片
// @Description  返回所有照片列表(按 createTime desc),支持排序覆盖
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Photo}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/latest [get]
func (p *PhotoHandler) ListPhoto(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "createTime,desc")
	}
	photos, err := p.PhotoService.List(ctx, &sort)
	if err != nil {
		return nil, err
	}
	return p.PhotoService.ConvertToDTOs(ctx, photos), nil
}

// PagePhotos godoc
// @Summary      分页查询照片
// @Description  支持排序与分页的照片查询
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        page  query     int        false  "页码(从0开始)"  example(0)
// @Param        size  query     int        false  "每页数量"        example(10)
// @Param        sort  query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.Photo}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos [get]
func (p *PhotoHandler) PagePhotos(ctx *gin.Context) (interface{}, error) {
	type Param struct {
		param.Page
		param.Sort
	}
	param := Param{}
	err := ctx.ShouldBindQuery(&param)
	if err != nil {
		return nil, xerr.WithMsg(err, "parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(param.Fields) == 0 {
		param.Fields = append(param.Fields, "createTime,desc")
	}
	photos, totalCount, err := p.PhotoService.Page(ctx, param.Page, &param.Sort)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(p.PhotoService.ConvertToDTOs(ctx, photos), totalCount, param.Page), nil
}

// GetPhotoByID godoc
// @Summary      根据ID获取照片
// @Description  返回指定 ID 的照片详情
// @Tags         Admin.Photo
// @Produce      json
// @Param        id  path     int  true  "照片ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Photo}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/{id} [get]
func (p *PhotoHandler) GetPhotoByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	photo, err := p.PhotoService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.PhotoService.ConvertToDTO(ctx, photo), nil
}

// CreatePhoto godoc
// @Summary      创建照片
// @Description  创建一个新照片,返回创建后的详情
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        photo  body     param.Photo  true  "照片参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Photo}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos [post]
func (p *PhotoHandler) CreatePhoto(ctx *gin.Context) (interface{}, error) {
	photoParam := &param.Photo{}
	err := ctx.ShouldBindJSON(photoParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	photo, err := p.PhotoService.Create(ctx, photoParam)
	if err != nil {
		return nil, err
	}
	return p.PhotoService.ConvertToDTO(ctx, photo), nil
}

// CreatePhotoBatch godoc
// @Summary      批量创建照片
// @Description  根据照片参数列表批量创建照片
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        photos  body     []param.Photo  true  "照片参数列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Photo}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/batch [post]
func (p *PhotoHandler) CreatePhotoBatch(ctx *gin.Context) (interface{}, error) {
	photosParam := make([]*param.Photo, 0)
	err := ctx.ShouldBindJSON(&photosParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	photos, err := p.PhotoService.CreateBatch(ctx, photosParam)
	if err != nil {
		return nil, err
	}
	return p.PhotoService.ConvertToDTOs(ctx, photos), nil
}

// UpdatePhoto godoc
// @Summary      更新照片
// @Description  根据照片ID更新照片信息
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        id     path     int          true  "照片ID"  example(1)
// @Param        photo  body     param.Photo  true  "照片参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Photo}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/{id} [put]
func (p *PhotoHandler) UpdatePhoto(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	photoParam := &param.Photo{}
	err = ctx.ShouldBindJSON(photoParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	photo, err := p.PhotoService.Update(ctx, id, photoParam)
	if err != nil {
		return nil, err
	}
	return p.PhotoService.ConvertToDTO(ctx, photo), nil
}

func (p *PhotoHandler) DeletePhoto(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return nil, p.PhotoService.Delete(ctx, id)
}

// DeletePhotoBatch godoc
// @Summary      批量删除照片
// @Description  根据照片ID列表批量删除照片
// @Tags         Admin.Photo
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "照片ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/batch [delete]
func (p *PhotoHandler) DeletePhotoBatch(ctx *gin.Context) (interface{}, error) {
	photosParam := make([]int32, 0)
	err := ctx.ShouldBindJSON(&photosParam)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	for _, id := range photosParam {
		err := p.PhotoService.Delete(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// ListPhotoTeams godoc
// @Summary      查询照片分组
// @Description  返回所有照片的分组(team)列表
// @Tags         Admin.Photo
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/photos/teams [get]
func (p *PhotoHandler) ListPhotoTeams(ctx *gin.Context) (interface{}, error) {
	return p.PhotoService.ListTeams(ctx)
}
