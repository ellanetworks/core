import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import AppRouter from "./router";
import { SnackbarProvider } from "./contexts/SnackbarContext";
import "./globals.scss";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <SnackbarProvider>
        <AppRouter />
      </SnackbarProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
