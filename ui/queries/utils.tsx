export const HTTPStatus = (code: number): string => {
  const map: { [key: number]: string } = {
    400: "Bad Request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not Found",
    409: "Conflict",
    500: "Internal Server Error",
  };
  if (!(code in map)) {
    throw new Error("code not recognized: " + code);
  }
  return map[code];
};
