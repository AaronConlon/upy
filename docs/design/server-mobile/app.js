// 设计稿只读取本地演示内容，不连接生产服务。
const current = new URLSearchParams(location.search).get('screen');
if (current) {
  document.body.classList.add('single');
  const target = document.querySelector(`[data-screen="${['overview', 'project', 'logs'].includes(current) ? current : 'overview'}"]`);
  target.classList.add('active');
}
document.querySelectorAll('[data-copy]').forEach(button => button.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(button.dataset.copy);
    button.setAttribute('aria-label', '任务 ID 已复制');
    button.innerHTML = '<span aria-live="polite">已复制</span>';
  } catch {
    button.innerHTML = '<span role="alert">复制失败</span>';
  }
}));
document.querySelector('#export').addEventListener('click', () => {
  const text = Array.from(document.querySelectorAll('.logline')).map(line => line.innerText).join('\n');
  const url = URL.createObjectURL(new Blob([text], {type: 'text/plain;charset=utf-8'}));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = 'microdraft-20261020-143208-demo.log';
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});
