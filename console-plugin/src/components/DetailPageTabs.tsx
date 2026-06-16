import { Link, useLocation } from 'react-router-dom-v5-compat';

export type DetailPageTab = {
  id: string;
  label: string;
  pathname: string;
  isActive: (pathname: string) => boolean;
};

type DetailPageTabsProps = {
  tabs: DetailPageTab[];
  search: string;
};

/** Console-style horizontal tabs using react-router Link so query params (e.g. ?ns=) are preserved. */
export function DetailPageTabs({ tabs, search }: DetailPageTabsProps) {
  const location = useLocation();

  return (
    <div className="co-m-horizontal-nav">
      <ul className="pf-v6-c-tabs pf-m-page-insets" role="tablist">
        {tabs.map((tab) => {
          const isCurrent = tab.isActive(location.pathname);

          return (
            <li
              key={tab.id}
              className={`pf-v6-c-tabs__item${isCurrent ? ' pf-m-current' : ''}`}
              role="presentation"
            >
              <Link
                to={{ pathname: tab.pathname, search }}
                role="tab"
                className="pf-v6-c-tabs__link"
                aria-selected={isCurrent}
              >
                <span className="pf-v6-c-tabs__item-text">{tab.label}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
