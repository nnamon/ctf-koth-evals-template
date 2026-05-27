import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/aesthetic.css";
import "./styles/app.css";
import { initTheme } from "./theme";
import App from "./App";

initTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
