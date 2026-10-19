// 眼睛坐标取自 1280×1280 的无瞳孔原图；所有绘制都使用原图坐标。
(() => {
  const host = document.querySelector(".mascot-motion");
  const canvas = host?.querySelector("canvas");
  const image = host?.querySelector("img");
  const ctx = canvas?.getContext("2d");
  if (!ctx) {
    if (image) image.src = "./assets/gopher.png";
    return;
  }
  const size = 1280;
  const eyes = [
    { x: 684, y: 313, rx: 82, ry: 101, rotation: 0.25, px: 30, py: 43 },
    { x: 922, y: 367, rx: 64, ry: 89, rotation: 0.25, px: 25, py: 36 },
  ];
  const motion = matchMedia("(prefers-reduced-motion: reduce)");
  const fine = matchMedia("(any-hover: hover) and (any-pointer: fine)");
  let pointer = null;
  let frame = 0;
  let visible = true;
  let offsets = eyes.map(() => ({ x: 0, y: 0 }));

  // 反解 GSAP 的平移、旋转和变换原点，避免用旋转后的外接矩形定位眼睛。
  function localPointer() {
    if (!pointer) return null;
    const style = getComputedStyle(host);
    const width = host.clientWidth;
    const height = host.clientHeight;
    if (!width || !height) return null;
    const [ox, oy] = style.transformOrigin.split(" ").map(parseFloat);
    const transform = new DOMMatrix(
      style.transform === "none" ? undefined : style.transform,
    );
    const matrix = new DOMMatrix()
      .translate(ox, oy)
      .multiply(transform)
      .translate(-ox, -oy);
    const corners = [
      [0, 0],
      [width, 0],
      [0, height],
      [width, height],
    ].map(([x, y]) => matrix.transformPoint({ x, y }));
    const box = host.getBoundingClientRect();
    const left = box.left - Math.min(...corners.map((p) => p.x));
    const top = box.top - Math.min(...corners.map((p) => p.y));
    const point = matrix
      .inverse()
      .transformPoint({ x: pointer.x - left, y: pointer.y - top });
    return { x: (point.x * size) / width, y: (point.y * size) / height };
  }

  function draw() {
    ctx.clearRect(0, 0, size, size);
    const point = !motion.matches && fine.matches ? localPointer() : null;
    eyes.forEach((eye, index) => {
      const cos = Math.cos(eye.rotation);
      const sin = Math.sin(eye.rotation);
      const dx = point ? point.x - eye.x : 0;
      const dy = point ? point.y - eye.y : 0;
      // 将视线限制在保守的内椭圆里，眼珠和高光均不会压住眼白轮廓。
      const maxX = eye.rx - eye.px - 12;
      const maxY = eye.ry - eye.py - 12;
      let x = (dx * cos + dy * sin) * 0.18;
      let y = (-dx * sin + dy * cos) * 0.18;
      const distance = Math.hypot(x / maxX, y / maxY);
      if (distance > 1) {
        x /= distance;
        y /= distance;
      }
      const offset = offsets[index];
      const easing = motion.matches || !fine.matches ? 1 : 0.22;
      offset.x += (x - offset.x) * easing;
      offset.y += (y - offset.y) * easing;
      ctx.save();
      ctx.translate(eye.x, eye.y);
      ctx.rotate(eye.rotation);
      ctx.beginPath();
      ctx.ellipse(0, 0, eye.rx - 5, eye.ry - 5, 0, 0, Math.PI * 2);
      ctx.clip();
      ctx.fillStyle = "#071119";
      ctx.beginPath();
      ctx.ellipse(offset.x, offset.y, eye.px, eye.py, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "#fff";
      ctx.beginPath();
      ctx.ellipse(
        offset.x + eye.px * 0.3,
        offset.y - eye.py * 0.42,
        eye.px * 0.26,
        eye.py * 0.23,
        0,
        0,
        Math.PI * 2,
      );
      ctx.fill();
      ctx.restore();
    });
  }

  function tick() {
    frame = 0;
    draw();
    // 角色持续浮动时仍需更新坐标；离屏、后台、触屏及减少动态模式不循环绘制。
    if (visible && !document.hidden && !motion.matches && fine.matches) {
      frame = requestAnimationFrame(tick);
    }
  }
  function refresh() {
    cancelAnimationFrame(frame);
    frame = 0;
    if (visible && !document.hidden) tick();
  }
  function resize() {
    const pixels = Math.max(
      1,
      Math.round(host.clientWidth * (devicePixelRatio || 1)),
    );
    canvas.width = pixels;
    canvas.height = pixels;
    ctx.setTransform(pixels / size, 0, 0, pixels / size, 0, 0);
    draw();
  }
  const resizeObserver = new ResizeObserver(resize);
  resizeObserver.observe(host);
  const observer = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting;
    refresh();
  });
  observer.observe(host);
  window.addEventListener(
    "pointermove",
    (event) => {
      if (event.pointerType !== "mouse" && event.pointerType !== "pen") return;
      pointer = { x: event.clientX, y: event.clientY };
    },
    { passive: true },
  );
  document.documentElement.addEventListener("pointerleave", () => {
    pointer = null;
  });
  window.addEventListener("blur", () => {
    pointer = null;
  });
  document.addEventListener("visibilitychange", refresh);
  motion.addEventListener("change", refresh);
  fine.addEventListener("change", refresh);
  window.addEventListener("resize", resize, { passive: true });
  window.addEventListener("pagehide", () => cancelAnimationFrame(frame));
  window.addEventListener("pageshow", refresh);
  resize();
  refresh();
})();
