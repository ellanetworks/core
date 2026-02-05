import type { Metadata } from "next";
import "./globals.scss";

export const metadata: Metadata = {
  title: "Ella Core",
  description: "A private mobile core network",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <head>
        <link rel="icon" href="/favicon.ico" sizes="any" />
      </head>
      <body>{children}</body>
    </html>
  );
}
