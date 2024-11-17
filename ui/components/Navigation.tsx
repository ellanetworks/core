import React, { FC, MouseEvent, useState } from "react";
import { Button, Icon } from "@canonical/react-components";
import classnames from "classnames";
import Logo from "./Logo";
import { usePathname } from "next/navigation";

const Navigation: FC = () => {
  const pathname = usePathname();
  const [isCollapsed, setCollapsed] = useState(false);

  const softToggleMenu = () => {
    if (window.innerWidth < 620) {
      setCollapsed((prev) => !prev);
    }
  };

  const hardToggleMenu = (e: MouseEvent<HTMLElement>) => {
    setCollapsed((prev) => !prev);
    e.stopPropagation();
  };

  return (
    <>
      <header className="l-navigation-bar">
        <div className="p-panel is-dark">
          <div className="p-panel__header">
            <Logo />
            <div className="p-panel__controls">
              <Button
                dense
                className="p-panel__toggle"
                onClick={hardToggleMenu}
              >
                Menu
              </Button>
            </div>
          </div>
        </div>
      </header>
      <nav
        aria-label="main navigation"
        className={classnames("l-navigation", {
          "is-collapsed": isCollapsed,
          "is-pinned": !isCollapsed,
        })}
        onClick={softToggleMenu}
      >
        <div className="l-navigation__drawer">
          <div className="p-panel is-dark">
            <div className="p-panel__header is-sticky">
              <Logo />
              <div className="p-panel__controls u-hide--medium u-hide--large">
                <Button
                  appearance="base"
                  hasIcon
                  className="u-no-margin"
                  aria-label="close navigation"
                  onClick={hardToggleMenu}
                >
                  <Icon name="close" />
                </Button>
              </div>
            </div>
            <div className="p-panel__content">
              <div className="p-side-navigation--icons is-dark">
                <ul className="p-side-navigation__list sidenav-top-ul">
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href="/network-slices"
                      title="Network Slices"
                      aria-current={
                        pathname === "/network-slices"
                          ? "page"
                          : undefined
                      }
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="containers"
                      />{" "}
                      Network slices
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href={`/device-groups`}
                      title={`Device Groups`}
                      aria-current={
                        pathname === "/device-groups" ? "page" : undefined
                      }
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="profiles"
                      />{" "}
                      Device Groups
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href={`/subscribers`}
                      title={`Subscribers`}
                      aria-current={
                        pathname === "/subscribers" ? "page" : undefined
                      }
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="user-group"
                      />{" "}
                      Subscribers
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href={`/upfs`}
                      title={`UPFs`}
                      aria-current={
                        pathname === "/upfs" ? "page" : undefined
                      }
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="machines"
                      />{" "}
                      UPFs
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href={`/radios`}
                      title={`Radios`}
                      aria-current={
                        pathname === "/radios" ? "page" : undefined
                      }
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="connected"
                      />{" "}
                      Radios
                    </a>
                  </li>
                </ul>
                <ul className="p-side-navigation__list sidenav-bottom-ul">
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href=""
                      target="_blank"
                      rel="noreferrer"
                      title="Documentation"
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="information"
                      />{" "}
                      Documentation
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href="https://github.com/yeastengine/ella"
                      target="_blank"
                      rel="noreferrer"
                      title="Source Code"
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="code"
                      />{" "}
                      Source Code
                    </a>
                  </li>
                  <li className="p-side-navigation__item">
                    <a
                      className="p-side-navigation__link"
                      href="https://github.com/yeastengine/ella/issues/new/choose"
                      target="_blank"
                      rel="noreferrer"
                      title="Report a bug"
                    >
                      <Icon
                        className="is-light p-side-navigation__icon"
                        name="submit-bug"
                      />{" "}
                      Report a bug
                    </a>
                  </li>
                </ul>
              </div>
            </div>
            <div className="sidenav-toggle-wrapper">
              <Button
                appearance="base"
                aria-label={`${isCollapsed ? "expand" : "collapse"
                  } main navigation`}
                hasIcon
                dense
                className="sidenav-toggle is-dark u-no-margin l-navigation-collapse-toggle u-hide--small"
                onClick={hardToggleMenu}
              >
                <Icon light name="sidebar-toggle" />
              </Button>
            </div>
          </div>
        </div>
      </nav>
    </>
  );
};

export default Navigation;
