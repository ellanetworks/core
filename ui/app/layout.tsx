import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Ella",
  description: "A 5G network",
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
