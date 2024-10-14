"use client";
import "../globals.scss";
import { Inter } from "next/font/google";
import React from "react";
import { Row } from "@canonical/react-components";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Navigation from "@/components/Navigation";

const inter = Inter({ subsets: ["latin"] });
const queryClient = new QueryClient();

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {

  return (
    <html lang="en">
      <head>
        <title>Ella</title>
        <link
          rel="shortcut icon"
          href="../favicon.ico"
          type="image/x-icon"
        />
      </head>
      <body className={inter.className}>
        <div className="l-application" role="presentation">
          <Navigation />
          <main className="l-main">
            <div className="p-panel">
              <QueryClientProvider client={queryClient}>
                {children}
              </QueryClientProvider>
            </div>
            <footer className="l-footer--sticky p-strip--light">
              <Row>
                <p>
                  © 2024 Guillaume Belanger.
                </p>
              </Row>
            </footer>
          </main>
        </div>
      </body>
    </html>
  );
}
