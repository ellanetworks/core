import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Aether SD-Core",
  description: "Aether SD-Core NMS",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
