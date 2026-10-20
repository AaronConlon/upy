// 复制操作在非安全上下文中回退到选区复制；结果通过原按钮与读屏区域反馈。
const feedback = document.getElementById("action-feedback");
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext)
    return navigator.clipboard.writeText(text);
  const field = document.createElement("textarea");
  field.value = text;
  field.style.position = "fixed";
  field.style.opacity = "0";
  document.body.append(field);
  field.select();
  const ok = document.execCommand("copy");
  field.remove();
  if (!ok) throw new Error("copy failed");
}
document.querySelectorAll("[data-copy]").forEach((button) =>
  button.addEventListener("click", async () => {
    if (button.disabled) return;
    const original = button.innerHTML;
    button.disabled = true;
    try {
      const target = document.getElementById(button.dataset.copy);
      await copyText(target.textContent);
      button.textContent = "已复制";
      feedback.textContent = "已复制到剪贴板";
    } catch {
      button.textContent = "复制失败";
      feedback.textContent = "复制失败，请手动选择文本复制";
    } finally {
      setTimeout(() => {
        button.innerHTML = original;
        button.disabled = false;
      }, 1800);
    }
  }),
);
const output = document.getElementById("term-body");
const refresh = document.getElementById("refresh-log");
if (output && refresh) {
  let live = output.dataset.live === "true";
  let busy = false;
  let stopped = false;
  const state = document.getElementById("live-state");
  async function refreshLog(manual) {
    if (
      busy ||
      stopped ||
      (!manual &&
        (!live || document.hidden || window.getSelection()?.toString()))
    )
      return;
    busy = true;
    refresh.disabled = true;
    refresh.setAttribute("aria-busy", "true");
    try {
      const url = new URL(location.href);
      url.search = "?format=json";
      const response = await fetch(url, {
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      if (response.redirected) {
        stopped = true;
        state.textContent = "会话已过期，请重新登录";
        const login = document.createElement("a");
        login.href = "/";
        login.textContent = " 重新登录";
        state.append(login);
        return;
      }
      if (!response.ok) throw new Error("request failed");
      const data = await response.json();
      // HTML 由服务端 RenderAnsiToHTML 转义，不插入日志原文。
      const atBottom =
        output.scrollHeight - output.scrollTop - output.clientHeight < 32;
      output.innerHTML = data.html;
      if (atBottom) output.scrollTop = output.scrollHeight;
      const status = document.getElementById("task-status");
      status.className = "status " + data.status_class;
      // 状态图标与文字同样由受控的服务端模板生成。
      status.innerHTML = data.status_html;
      document.getElementById("task-duration").textContent =
        data.duration || "—";
      document.getElementById("exit-code").textContent = data.exit_label;
      document.getElementById("log-truncated").hidden = !data.truncated;
      live = data.live;
      state.textContent = live ? "每 3 秒刷新" : "日志已更新";
    } catch {
      state.textContent = "更新失败，可点击刷新重试";
    } finally {
      busy = false;
      refresh.disabled = false;
      refresh.removeAttribute("aria-busy");
    }
  }
  refresh.addEventListener("click", () => refreshLog(true));
  setInterval(() => refreshLog(false), 3000);
}

// 安装入口按浏览器能力显示；Safari 的安装路径使用原生分享菜单。
const installPanel = document.getElementById("pwa-install");
const installButton = document.getElementById("install-app");
const standalone =
  matchMedia("(display-mode: standalone)").matches ||
  navigator.standalone === true;
let installPrompt;
if (!standalone && window.isSecureContext) {
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  if (ios) {
    installPanel.hidden = false;
    installButton.addEventListener("click", () => {
      document.getElementById("ios-install-help").hidden = false;
    });
  } else {
    window.addEventListener("beforeinstallprompt", (event) => {
      event.preventDefault();
      installPrompt = event;
      installPanel.hidden = false;
    });
    installButton.addEventListener("click", async () => {
      if (!installPrompt) return;
      installButton.disabled = true;
      try {
        await installPrompt.prompt();
        await installPrompt.userChoice;
      } finally {
        installPrompt = null;
        installPanel.hidden = true;
        installButton.disabled = false;
      }
    });
  }
}
window.addEventListener("appinstalled", () => {
  installPanel.hidden = true;
  installPrompt = null;
});

if ("serviceWorker" in navigator && window.isSecureContext) {
  let applyingUpdate = false;
  let waitingWorker;
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (applyingUpdate) location.reload();
  });
  navigator.serviceWorker
    .register("/sw.js", { scope: "/", updateViaCache: "none" })
    .then((registration) => {
      const offerUpdate = () => {
        if (!registration.waiting || !navigator.serviceWorker.controller)
          return;
        waitingWorker = registration.waiting;
        document.getElementById("pwa-update").hidden = false;
      };
      offerUpdate();
      registration.addEventListener("updatefound", () => {
        const worker = registration.installing;
        worker?.addEventListener("statechange", () => {
          if (worker.state === "installed") offerUpdate();
        });
      });
      document.getElementById("update-app").addEventListener("click", () => {
        if (!waitingWorker) return;
        applyingUpdate = true;
        document.getElementById("update-app").disabled = true;
        waitingWorker.postMessage({ type: "ACTIVATE_UPDATE" });
      });
      // 从后台回来时检查 worker 版本，不强制刷新正在阅读的日志。
      document.addEventListener("visibilitychange", () => {
        if (!document.hidden) registration.update().catch(() => {});
      });
    })
    .catch(() => {
      // 普通在线控制台不依赖 PWA 注册成功。
    });
}
