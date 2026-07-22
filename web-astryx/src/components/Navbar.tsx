import { TopNav } from '@astryxdesign/core/TopNav';
import { TopNavHeading } from '@astryxdesign/core/TopNav';
import { TopNavItem } from '@astryxdesign/core/TopNav';
import { useLocation } from 'react-router-dom';

export function Navbar() {
  const location = useLocation();
  const currentPath = location.pathname;

  const isShows = currentPath === '/shows' || currentPath.startsWith('/shows/');
  const isActivity = currentPath === '/activity';
  const isSettings = currentPath === '/settings';

  return (
    <TopNav
      label="Mizu navigation"
      heading={
        <TopNavHeading heading="Mizu" headingHref="/shows" />
      }
      startContent={
        <>
          <TopNavItem label="Anime" href="/shows" isSelected={isShows} />
          <TopNavItem label="Activity" href="/activity" isSelected={isActivity} />
          <TopNavItem label="Settings" href="/settings" isSelected={isSettings} />
        </>
      }
    />
  );
}
