// 使用 Clipboard API；权限不可用时选中命令，保留手动复制路径。
const status = document.querySelector("#copy-status");
for (const button of document.querySelectorAll("[data-copy]")) {
  button.addEventListener("click", async () => {
    const source = document.getElementById(button.dataset.copy);
    button.disabled = true;
    try {
      await navigator.clipboard.writeText(source.textContent.trim());
      button.textContent = "已复制";
      status.textContent = "安装命令已复制到剪贴板。";
    } catch {
      const range = document.createRange();
      range.selectNodeContents(source);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      button.textContent = "请手动复制";
      status.textContent =
        "无法自动复制，已选中安装命令，请使用系统复制快捷键。";
    } finally {
      button.disabled = false;
      window.setTimeout(() => {
        button.textContent = "复制命令";
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
    media.add("(prefers-reduced-motion: no-preference)", () => {
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
          ".mascot-stage img",
          { x: -65, rotation: -6, opacity: 0, duration: 0.85 },
          0.25,
        );
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
