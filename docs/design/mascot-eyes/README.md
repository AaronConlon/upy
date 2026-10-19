# Hero 鼠标视线交互

日期：2026-10-19。

- 中英文 Hero 使用无瞳孔 PNG，原 Logo 继续用于 favicon 和其他原有引用。
- Canvas 叠在同一动画容器内，使用原图坐标绘制两个椭圆瞳孔与高光。
- DOMMatrix 反解角色平移、旋转和变换原点，视线由整个窗口中的鼠标位置驱动。
- 眼珠限制在眼白内部的保守椭圆内，并额外裁切，移动采用平滑插值。
- Canvas backing store 随显示尺寸与 devicePixelRatio 调整。
- 离屏、后台时暂停循环；鼠标离开窗口或窗口失焦时回中；触屏和减少动态模式保持中立视线。
- Canvas 不可用时回退到有瞳孔的原 Logo。
- GSAP 入场、巡航和微视差统一作用于图片与 Canvas 的父容器；动态偏好切换时清理延迟注册的动画和监听器。

## 验证

真实 Chromium：

1. 连续三轮移动鼠标至窗口左上与右下，读取 Canvas 黑色像素重心，确认瞳孔随方向移动。
2. 320 / 375 / 768 / 1440px 下图片和 Canvas 外接矩形保持对齐。
3. 连续三次切换减少动态偏好，确认静态模式鼠标移动不会改变 Canvas；恢复偏好后 ScrollTrigger 正常启用。
4. 英文与中文页面均显示眼珠；模拟触屏设备确认无精细指针、无横向溢出，高像素密度 Canvas 正常。
5. JavaScript 语法与 diff 空白检查通过。浏览器没有运行错误；像素读取测试触发 Canvas 性能建议，该提示来自测试读取操作，生产代码不读取像素。

截图：desktop.png、desktop-detail.png、chinese-detail.png、mobile-detail.png。
