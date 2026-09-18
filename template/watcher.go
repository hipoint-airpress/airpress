package template

import (
	"time"

	"go.uber.org/zap"
)

// watchDebounce 合并编辑器保存产生的连续文件事件，避免反复重载
const watchDebounce = 100 * time.Millisecond

func (t *Template) Watch() {
	var (
		timer  *time.Timer
		timerC <-chan time.Time
	)

	for {
		select {
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			if !t.isTemplateEvent(event) {
				continue
			}
			t.logger.Debug("template file changed",
				zap.String("file", event.Name),
				zap.String("op", event.Op.String()))

			// 重新计时：timerC 为 nil 时该分支永不触发
			timer = time.NewTimer(watchDebounce)
			timerC = timer.C
		case <-timerC:
			timer = nil
			timerC = nil
			if err := t.Reload(nil); err != nil {
				t.logger.Error("reload template error", zap.Error(err))
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.logger.Error("file watcher error:", zap.Error(err))
		}
	}
}