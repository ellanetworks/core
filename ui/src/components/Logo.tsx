import React from "react";

export default function Logo({
  width = 50,
  height = 50,
}: {
  width?: number;
  height?: number;
}) {
  return (
    <img
      src="/logo.svg"
      alt="Ella Core Logo"
      width={width}
      height={height}
    />
  );
}
