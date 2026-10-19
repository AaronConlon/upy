// 使用 Clipboard API；权限不可用时选中命令，保留手动复制路径。
const status = document.querySelector("#copy-status");
const isZh = document.documentElement.lang.startsWith("zh");
const textCopy = isZh ? "复制命令" : "Copy";
const textCopied = isZh ? "已复制" : "Copied!";
const textManual = isZh ? "请手动复制" : "Copy manually";
const textSuccessMsg = isZh
  ? "安装命令已复制到剪贴板。"
  : "Install command copied to clipboard.";
const textFailMsg = isZh
  ? "无法自动复制，已选中安装命令，请使用系统复制快捷键。"
  : "Could not copy automatically. Command selected, please use Ctrl+C / Cmd+C.";

for (const button of document.querySelectorAll("[data-copy]")) {
  button.addEventListener("click", async () => {
    const source = document.getElementById(button.dataset.copy);
    button.disabled = true;
    try {
      await navigator.clipboard.writeText(source.textContent.trim());
      button.textContent = textCopied;
      status.textContent = textSuccessMsg;
    } catch {
      const range = document.createRange();
      range.selectNodeContents(source);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      button.textContent = textManual;
      status.textContent = textFailMsg;
    } finally {
      button.disabled = false;
      window.setTimeout(() => {
        button.textContent = textCopy;
      }, 3000);
    }
  });
}

// 渐进增强：内容默认可见，GSAP 不可用时仍能完整阅读与复制命令。
if (window.gsap && window.ScrollTrigger) {
  gsap.registerPlugin(ScrollTrigger);
  let media;
  const startMotion = () => {
    media = gsap.matchMedia();
    media.add("(prefers-reduced-motion: no-preference)", (context) => {
      const cleanups = [];
      const entry = gsap.timeline({ defaults: { ease: "power2.out" } });
      const inkLine = document.querySelector(".construction path");
      const lineLength = inkLine.getTotalLength();
      entry.fromTo(
        inkLine,
        { strokeDasharray: lineLength, strokeDashoffset: lineLength },
        {
          strokeDashoffset: 0,
          duration: 0.8,
          clearProps: "strokeDasharray,strokeDashoffset",
        },
        0,
      );
      entry
        .from(".construction", { opacity: 0, duration: 0.5 })
        .from(
          ".dimension",
          { scaleX: 0, transformOrigin: "center", duration: 0.6 },
          0.15,
        )
        .from(".crosses", { opacity: 0, duration: 0.35 }, 0.3)
        .from(
          ".hero-copy h1 > span",
          { y: 22, opacity: 0, stagger: 0.1, duration: 0.65 },
          0.1,
        )
        .from(
          ".mascot-motion",
          { x: -65, rotation: -6, opacity: 0, duration: 0.85 },
          0.25,
        )
        .add(() => context.startCruise());
      // 命名上下文让延迟创建的巡航动画和事件动画也能随偏好切换撤销。
      context.add("startCruise", () => {
        const cruise = gsap.to(".mascot-motion", {
          y: -8,
          x: 4,
          rotation: 1.6,
          duration: 3.2,
          repeat: -1,
          yoyo: true,
          ease: "sine.inOut",
        });
        gsap.to(".construction circle", {
          rotation: 360,
          transformOrigin: "280px 214px",
          duration: 90,
          repeat: -1,
          ease: "none",
        });
        gsap.to(".crosses path", {
          opacity: 0.5,
          duration: 2.6,
          repeat: -1,
          yoyo: true,
          ease: "sine.inOut",
          stagger: 0.15,
        });
        const stage = document.querySelector(".drawing-field");
        const mascot = document.querySelector(".mascot-motion");
        context.add("moveMascot", (event) => {
          cruise.pause();
          const rect = stage.getBoundingClientRect();
          const x = (event.clientX - rect.left) / rect.width - 0.5;
          const y = (event.clientY - rect.top) / rect.height - 0.5;
          gsap.to(mascot, {
            x: x * 24,
            y: y * 16,
            rotation: x * 3.5,
            duration: 0.45,
            ease: "power2.out",
            overwrite: "auto",
          });
        });
        context.add("leaveMascot", () => {
          gsap.to(mascot, {
            x: 0,
            y: 0,
            rotation: 0,
            duration: 0.9,
            ease: "sine.out",
            overwrite: "auto",
            onComplete: () => cruise.invalidate().restart(),
          });
        });
        stage.addEventListener("mousemove", context.moveMascot);
        stage.addEventListener("mouseleave", context.leaveMascot);
        cleanups.push(() => {
          stage.removeEventListener("mousemove", context.moveMascot);
          stage.removeEventListener("mouseleave", context.leaveMascot);
        });
      });
      for (const section of document.querySelectorAll(
        ".installation, .section-heading, .step, .deploy-types, .closing-inner",
      )) {
        gsap.from(section, {
          y: 22,
          opacity: 0,
          duration: 0.6,
          ease: "power2.out",
          immediateRender: false,
          scrollTrigger: { trigger: section, start: "top 90%", once: true },
        });
      }
      // 流程线随原生滚动推进，无 pin、无滚轮接管；节点只表达阅读进度。
      gsap.from(".workflow-progress", {
        scaleY: 0,
        ease: "none",
        scrollTrigger: {
          trigger: ".workflow-track",
          start: "top 65%",
          end: "bottom 65%",
          scrub: 0.4,
        },
      });
      for (const step of document.querySelectorAll("[data-step]")) {
        ScrollTrigger.create({
          trigger: step,
          start: "top 65%",
          end: "bottom 65%",
          toggleClass: "is-active",
        });
      }
      return () => cleanups.forEach((cleanup) => cleanup());
    });
  };
  startMotion();
  // bfcache 恢复时重建上下文，离开页面时撤销内联动画状态与滚动监听。
  window.addEventListener("pagehide", () => {
    media?.revert();
    media = null;
  });
  window.addEventListener("pageshow", (event) => {
    if (event.persisted && !media) startMotion();
  });
  document.fonts.ready.then(() => ScrollTrigger.refresh());
}
