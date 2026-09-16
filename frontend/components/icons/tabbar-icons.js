import { Svg, Path, Rect, Line } from 'react-native-svg';
import { colors } from '@/theme/palette';

export const MyTaskIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M14.825 18.9581H5.17499C2.89166 18.9581 1.04166 17.0998 1.04166 14.8165V8.64148C1.04166 7.50815 1.74166 6.08315 2.64166 5.38315L7.13332 1.88315C8.48332 0.833149 10.6417 0.783149 12.0417 1.76648L17.1917 5.37481C18.1833 6.06648 18.9583 7.54982 18.9583 8.75815V14.8248C18.9583 17.0998 17.1083 18.9581 14.825 18.9581ZM7.89999 2.86648L3.40832 6.36648C2.81666 6.83315 2.29166 7.89148 2.29166 8.64148V14.8165C2.29166 16.4081 3.58332 17.7081 5.17499 17.7081H14.825C16.4167 17.7081 17.7083 16.4165 17.7083 14.8248V8.75815C17.7083 7.95815 17.1333 6.84981 16.475 6.39982L11.325 2.79148C10.375 2.12482 8.80832 2.15815 7.89999 2.86648Z"
        fill={colors.iconTabInactive}
      />
      <Path
        d="M10 15.625C9.65833 15.625 9.375 15.3417 9.375 15V12.5C9.375 12.1583 9.65833 11.875 10 11.875C10.3417 11.875 10.625 12.1583 10.625 12.5V15C10.625 15.3417 10.3417 15.625 10 15.625Z"
        fill={colors.iconTabInactive}
      />
    </Svg>
  );
};

export const MyTaskActiveIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M16.7 5.68373L11.9 2.3254C10.5917 1.40873 8.58332 1.45873 7.32499 2.43373L3.14999 5.69206C2.31666 6.34206 1.65833 7.6754 1.65833 8.7254V14.4754C1.65833 16.6004 3.38332 18.3337 5.50832 18.3337H14.4917C16.6167 18.3337 18.3417 16.6087 18.3417 14.4837V8.83373C18.3417 7.70873 17.6167 6.3254 16.7 5.68373ZM10.625 15.0004C10.625 15.3421 10.3417 15.6254 9.99999 15.6254C9.65832 15.6254 9.37499 15.3421 9.37499 15.0004V12.5004C9.37499 12.1587 9.65832 11.8754 9.99999 11.8754C10.3417 11.8754 10.625 12.1587 10.625 12.5004V15.0004Z"
        fill={colors.brand}
      />
    </Svg>
  );
};

export const PostTabIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M10 18.3337C14.5834 18.3337 18.3334 14.5837 18.3334 10.0003C18.3334 5.41699 14.5834 1.66699 10 1.66699C5.41669 1.66699 1.66669 5.41699 1.66669 10.0003C1.66669 14.5837 5.41669 18.3337 10 18.3337Z"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Path
        d="M6.66669 10H13.3334"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Path
        d="M10 13.3337V6.66699"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
  );
};

export const PostTabActiveIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M9.99999 1.6665C5.40832 1.6665 1.66666 5.40817 1.66666 9.99984C1.66666 14.5915 5.40832 18.3332 9.99999 18.3332C14.5917 18.3332 18.3333 14.5915 18.3333 9.99984C18.3333 5.40817 14.5917 1.6665 9.99999 1.6665ZM13.3333 10.6248H10.625V13.3332C10.625 13.6748 10.3417 13.9582 9.99999 13.9582C9.65832 13.9582 9.37499 13.6748 9.37499 13.3332V10.6248H6.66666C6.32499 10.6248 6.04166 10.3415 6.04166 9.99984C6.04166 9.65817 6.32499 9.37484 6.66666 9.37484H9.37499V6.6665C9.37499 6.32484 9.65832 6.0415 9.99999 6.0415C10.3417 6.0415 10.625 6.32484 10.625 6.6665V9.37484H13.3333C13.675 9.37484 13.9583 9.65817 13.9583 9.99984C13.9583 10.3415 13.675 10.6248 13.3333 10.6248Z"
        fill={colors.brand}
      />
    </Svg>
  );
};

export const TasksTabIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Rect
        x="3.75"
        y="4.16667"
        width="12.5"
        height="13.3333"
        rx="2"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
      />
      <Rect
        x="7.5"
        y="2.5"
        width="5"
        height="3.33333"
        rx="1"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
      />
      <Line
        x1="6.66667"
        y1="9.58333"
        x2="13.3333"
        y2="9.58333"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
      />
      <Line
        x1="6.66667"
        y1="12.5"
        x2="13.3333"
        y2="12.5"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
      />
      <Line
        x1="6.66667"
        y1="15.4167"
        x2="10.8333"
        y2="15.4167"
        stroke={colors.iconTabInactive}
        strokeWidth="1.25"
        strokeLinecap="round"
      />
    </Svg>
  );
};

export const TasksTabActiveIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Rect x="3.75" y="4.16667" width="12.5" height="13.3333" rx="2" fill={colors.brand} />
      <Rect x="7.5" y="2.5" width="5" height="3.33333" rx="1" fill={colors.brand} />
      <Line
        x1="6.66667"
        y1="9.58333"
        x2="13.3333"
        y2="9.58333"
        stroke="white"
        strokeWidth="1.25"
        strokeLinecap="round"
      />
      <Line
        x1="6.66667"
        y1="12.5"
        x2="13.3333"
        y2="12.5"
        stroke="white"
        strokeWidth="1.25"
        strokeLinecap="round"
      />
      <Line
        x1="6.66667"
        y1="15.4167"
        x2="10.8333"
        y2="15.4167"
        stroke="white"
        strokeWidth="1.25"
        strokeLinecap="round"
      />
    </Svg>
  );
};

export const ChatTabIcon = () => {
  return (
    <Svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M7.08334 8.75H12.9167"
        stroke={colors.iconTabInactive}
        stroke-width="1.25"
        stroke-miterlimit="10"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <Path
        d="M5.83332 15.3582H9.16666L12.875 17.8249C13.425 18.1916 14.1667 17.7999 14.1667 17.1332V15.3582C16.6667 15.3582 18.3333 13.6916 18.3333 11.1916V6.19157C18.3333 3.69157 16.6667 2.0249 14.1667 2.0249H5.83332C3.33332 2.0249 1.66666 3.69157 1.66666 6.19157V11.1916C1.66666 13.6916 3.33332 15.3582 5.83332 15.3582Z"
        stroke={colors.iconTabInactive}
        stroke-width="1.25"
        stroke-miterlimit="10"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </Svg>
  );
};

export const ChatActiveTabIcon = () => {
  return (
    <Svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M17 2.42969H7C4 2.42969 2 4.42969 2 7.42969V13.4297C2 16.4297 4 18.4297 7 18.4297H11L15.45 21.3897C16.11 21.8297 17 21.3597 17 20.5597V18.4297C20 18.4297 22 16.4297 22 13.4297V7.42969C22 4.42969 20 2.42969 17 2.42969ZM15.5 11.2497H8.5C8.09 11.2497 7.75 10.9097 7.75 10.4997C7.75 10.0897 8.09 9.74969 8.5 9.74969H15.5C15.91 9.74969 16.25 10.0897 16.25 10.4997C16.25 10.9097 15.91 11.2497 15.5 11.2497Z"
        fill={colors.brand}
      />
    </Svg>
  );
};

export const ProfileTabIcon = () => {
  return (
    <Svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M12.1601 10.87C12.0601 10.86 11.9401 10.86 11.8301 10.87C9.45006 10.79 7.56006 8.84 7.56006 6.44C7.56006 3.99 9.54006 2 12.0001 2C14.4501 2 16.4401 3.99 16.4401 6.44C16.4301 8.84 14.5401 10.79 12.1601 10.87Z"
        stroke={colors.iconTabInactive}
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <Path
        d="M7.15997 14.56C4.73997 16.18 4.73997 18.82 7.15997 20.43C9.90997 22.27 14.42 22.27 17.17 20.43C19.59 18.81 19.59 16.17 17.17 14.56C14.43 12.73 9.91997 12.73 7.15997 14.56Z"
        stroke={colors.iconTabInactive}
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </Svg>
  );
};

export const ProfileActiveTabIcon = () => {
  return (
    <Svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
      <Path
        d="M12.1601 10.87C12.0601 10.86 11.9401 10.86 11.8301 10.87C9.45006 10.79 7.56006 8.84 7.56006 6.44C7.56006 3.99 9.54006 2 12.0001 2C14.4501 2 16.4401 3.99 16.4401 6.44C16.4301 8.84 14.5401 10.79 12.1601 10.87Z"
        fill={colors.brand}
      />
      <Path
        d="M7.15997 14.56C4.73997 16.18 4.73997 18.82 7.15997 20.43C9.90997 22.27 14.42 22.27 17.17 20.43C19.59 18.81 19.59 16.17 17.17 14.56C14.43 12.73 9.91997 12.73 7.15997 14.56Z"
        fill={colors.brand}
      />
    </Svg>
  );
};
