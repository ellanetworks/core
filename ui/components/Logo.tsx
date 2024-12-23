import React from "react";
import Image from "next/image";

export default function Logo({ width = 100, height = 50 }: { width?: number; height?: number }) {
    return (
        <Image
            src="https://raw.githubusercontent.com/yeastengine/ella/main/logo.svg"
            alt="Ella Private Network Logo"
            width={width}
            height={height}
            priority
        />
    );
}
