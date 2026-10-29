import React from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App.jsx";
import { clearPendingRequests } from "./api.js";
import "./styles.css";

const root=createRoot(document.getElementById("root"));
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
// A browser back/forward snapshot must not restore pasted account credentials.
window.addEventListener('pagehide',()=>{clearPendingRequests();root.unmount();});
window.addEventListener('pageshow',event=>{if(event.persisted)window.location.reload();});
