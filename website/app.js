// 使用 Clipboard API；权限不可用时选中命令，保留手动复制路径。
const status = document.querySelector('#copy-status');
for (const button of document.querySelectorAll('[data-copy]')) {
  button.addEventListener('click', async () => {
    const source = document.getElementById(button.dataset.copy);
    button.disabled = true;
    try {
      await navigator.clipboard.writeText(source.textContent.trim());
      button.textContent = '已复制';
      status.textContent = '安装命令已复制到剪贴板。';
    } catch {
      const range = document.createRange();
      range.selectNodeContents(source);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      button.textContent = '请手动复制';
      status.textContent = '无法自动复制，已选中安装命令，请使用系统复制快捷键。';
    } finally {
      button.disabled = false;
      window.setTimeout(() => { button.textContent = '复制命令'; }, 3000);
    }
  });
}
