import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App as AntdApp, ConfigProvider, theme } from "antd";
import zhCN from "antd/locale/zh_CN";
import App from "./App.jsx";
import "./App.css";

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: "#2dd4bf",
          colorInfo: "#38bdf8",
          colorSuccess: "#34d399",
          colorWarning: "#fbbf24",
          colorError: "#f87171",
          colorBgContainer: "#121a2b",
          colorBgLayout: "#070b14",
          colorBorder: "rgba(148, 163, 184, 0.16)",
          borderRadius: 10,
          fontFamily:
            '"Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
        },
        components: {
          Card: {
            headerBg: "transparent",
          },
          Button: {
            primaryShadow: "0 0 0 3px rgba(45, 212, 191, 0.16)",
          },
          Input: {
            activeBorderColor: "#2dd4bf",
            hoverBorderColor: "rgba(45, 212, 191, 0.55)",
          },
        },
      }}
    >
      <AntdApp style={{ minHeight: "100%" }}>
        <App />
      </AntdApp>
    </ConfigProvider>
  </StrictMode>
);
