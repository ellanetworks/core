import React, { FC } from "react";

import Image from "next/image";

const Logo: FC = () => {
  return (
    <a className="logo" href="/">
      <div className="logo-tag">
        <Image
          src="https://assets.ubuntu.com/v1/82818827-CoF_white.svg"
          alt="circle of friends"
          width={32}
          height={32}
          className="logo-image"
        />
      </div>
      <span className="logo-text p-heading--4">Ella</span>
    </a>
  );
};

export default Logo;
