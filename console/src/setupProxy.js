const { createProxyMiddleware } = require('http-proxy-middleware');

module.exports = function (app) {
  app.use(
    '/t',
    createProxyMiddleware({
      target: 'https://127.0.0.1:8900',
      changeOrigin: true,
      secure: false, // accept self-signed certs
    })
  );
};
