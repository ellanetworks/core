"use client";
import "../globals.scss";
import { Inter } from "next/font/google";
import React, { useState, useEffect } from "react";
import { checkBackendAvailable } from "@/utils/checkBackendAvailable";
import { List, Notification, Row } from "@canonical/react-components";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Navigation from "@/components/Navigation";
import PageContent from "@/components/PageContent";
import Loader from "@/components/Loader";

const inter = Inter({ subsets: ["latin"] });
const queryClient = new QueryClient();

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
  noLayout?: boolean;
}) {
  const [backendAvailable, setBackendAvailable] = useState<null | boolean>(
    null,
  );

  useEffect(() => {
    const fetchData = async () => {
      const isBackendAvailable = await checkBackendAvailable();
      setBackendAvailable(isBackendAvailable);
    };

    fetchData();
  }, []);

  return (
    <html lang="en">
      <head>
        <title>Aether SD-Core</title>
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
              {backendAvailable === null && <Loader text="Loading..." />}
              {backendAvailable === false && (
                <PageContent>
                  <Notification severity="negative" title="Error">
                    {"Backend not available"}
                  </Notification>
                </PageContent>
              )}
              {backendAvailable === true && (
                <QueryClientProvider client={queryClient}>
                  {children}
                </QueryClientProvider>
              )}
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
