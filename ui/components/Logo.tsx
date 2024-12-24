import React from "react";
import Image from "next/image";

export default function Logo({ width = 50, height = 50 }: { width?: number; height?: number }) {
    return (
        <Image
            src="https://raw.githubusercontent.com/yeastengine/ella-public/refs/heads/dev-logo/logo.png"
            alt="Ella Private Network Logo"
            width={width}
            height={height}
            unoptimized
            priority
        />
    );
}
