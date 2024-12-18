"use client";
import "../globals.scss";
import { Inter } from "next/font/google";
import React from "react";
import { List, Row } from "@canonical/react-components";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Navigation from "@/components/Navigation";

const inter = Inter({ subsets: ["latin"] });
const queryClient = new QueryClient();

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
  noLayout?: boolean;
}) {

  return (
    <html lang="en">
      <head>
        <title>Ella</title>
        <link
          rel="shortcut icon"
          href="https://assets.ubuntu.com/v1/49a1a858-favicon-32x32.png"
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
                  © 2023 Canonical Ltd. <a href="#">Ubuntu</a> and{" "}
                  <a href="#">Canonical</a> are registered trademarks of
                  Canonical Ltd.
                </p>
                <List
                  items={[
                    <a key="Legal information" href="https://ubuntu.com/legal">
                      Legal information
                    </a>,
                  ]}
                  middot
                />
              </Row>
            </footer>
          </main>
        </div>
      </body>
    </html>
  );
}
