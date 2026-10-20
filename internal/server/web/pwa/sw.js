// 只缓存公开白名单。认证页、项目、日志、API 与下载不进入 Cache Storage。
const CACHE = "__UPY_CACHE__";
const PUBLIC = [
  "/pwa/offline.html",
  "/pwa/icon.svg",
  "/pwa/icon-192.png",
  "/pwa/icon-512.png",
  "/pwa/apple-touch-icon.png",
];
self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(PUBLIC)));
});
self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys();
      await Promise.all(
        names
          .filter((name) => name.startsWith("upy-pwa-") && name !== CACHE)
          .map((name) => caches.delete(name)),
      );
      await self.clients.claim();
    })(),
  );
});
self.addEventListener("message", (event) => {
  if (event.data?.type === "ACTIVATE_UPDATE") self.skipWaiting();
});
self.addEventListener("fetch", (event) => {
  const request = event.request;
  const url = new URL(request.url);
  if (request.method !== "GET" || url.origin !== self.location.origin) return;
  if (PUBLIC.includes(url.pathname) && !url.search) {
    event.respondWith(
      caches
        .open(CACHE)
        .then(async (cache) => (await cache.match(request)) || fetch(request)),
    );
    return;
  }
  // 只有控制台文档导航可回退到无数据的离线页；JSON、下载与 POST 始终交给网络。
  const consolePage =
    url.pathname === "/" ||
    /^\/(overview|projects|logs)(\/|$)/.test(url.pathname);
  if (
    request.mode === "navigate" &&
    consolePage &&
    !url.searchParams.has("download") &&
    !url.searchParams.has("format")
  ) {
    event.respondWith(
      fetch(request, { cache: "no-store" })
        .then((response) => {
          if (response.status >= 500) throw new Error("server unavailable");
          return response;
        })
        .catch(async () => {
          return (await caches.open(CACHE)).match("/pwa/offline.html");
        }),
    );
  }
});
