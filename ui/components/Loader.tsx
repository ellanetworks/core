import React, { FC } from "react";
import { Spinner } from "@canonical/react-components";

interface Props {
  text?: string;
}

const Loader: FC<Props> = ({ text = "Loading..." }) => {
  return (
    <>
      <div className="u-sv3" />
      <div className="u-loader u-align-text--center">
        <Spinner text={text} />
      </div>
    </>
  );
};

export default Loader;
