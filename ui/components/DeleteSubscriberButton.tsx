"use client";
import { Button } from "@canonical/react-components";
import { useState } from "react";

type Props = {
  currentSubscribers: string[];
  imsi: string;
  refreshHandler: () => void;
};

const DeleteSubscriberButton = (props: Props) => {
  const [deleting, setDeleting] = useState<boolean>(false);

  const handleDeleteSubscriber = async () => {
    setDeleting(true);

    try {
      const response = await fetch(`/api/deleteSubscriber?imsi=${props.imsi}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ currentSubscribers: props.currentSubscribers }),
      });

      if (!response.ok) {
        throw new Error("Failed to delete subscriber");
      }

      setDeleting(false);
      props.refreshHandler();
    } catch (error) {
      console.error(error);
      setDeleting(false);
    }
  };

  return (
    <Button
      onClick={handleDeleteSubscriber}
      appearance="negative"
      className="u-no-margin--bottom"
      key={props.imsi}
      small={true}
      disabled={deleting}
    >
      {deleting ? "Deleting..." : "Delete"}
    </Button>
  );
};

export default DeleteSubscriberButton;
