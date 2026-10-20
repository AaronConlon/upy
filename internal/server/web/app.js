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
