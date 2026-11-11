<?php
/**
 * Plugin Name: Digitaal GemeenGoed — gemeenschap
 * Description: Neemt bij elke login de gegevens uit Keycloak (realm "gemeenschap") over in WordPress, BuddyPress, bbPress en CiviCRM. Keycloak is de bron; hier wordt niets terug geschreven.
 * Version: 0.1.0
 * Author: Digitaal GemeenGoed
 * License: EUPL-1.2
 *
 * Een must-use plugin: staat in wp-content/mu-plugins en kan niet worden
 * uitgezet. Werkt samen met de plugin OpenID Connect Generic (inloggen via
 * Keycloak); BuddyPress, bbPress en CiviCRM zijn optioneel: wat er niet is,
 * wordt overgeslagen.
 *
 * Wat er gebeurt bij elke login (de claims uit Keycloak):
 *  - groups (/netwerk/..., /site/...) → WordPress-rol, bbPress-rol en
 *    BuddyPress-ledentype (gebruiker, contributor, member, kerngroep);
 *  - organisatie, organization, functie, werkveld → gebruikersgegevens en
 *    BuddyPress-profielvelden (alleen-lezen: wijzigen doe je in Keycloak);
 *  - toestemming_mailing → CiviCRM-groep "Nieuwsbrief", met een activiteit
 *    met datum bij elke wijziging;
 *  - in CiviCRM: organisatiecontact met de relatie "medewerker van", en de
 *    netwerkgroepen als CiviCRM-groepen.
 */

defined( 'ABSPATH' ) || exit;

/*
 * Inloggen via Keycloak: de instellingen van OpenID Connect Generic als
 * constanten, uit de omgeving van de container (compose.yml). Een
 * must-use plugin laadt vóór de gewone plugins, dus de constanten staan er
 * op tijd. Zonder DG_KEYCLOAK_URL (bijvoorbeeld lokaal) gebeurt hier niets.
 */
if ( getenv( 'DG_KEYCLOAK_URL' ) && ! defined( 'OIDC_CLIENT_ID' ) ) {
	$dg_realm = rtrim( (string) getenv( 'DG_KEYCLOAK_URL' ), '/' ) . '/realms/' . ( getenv( 'DG_KEYCLOAK_REALM' ) ?: 'gemeenschap' );
	$dg_oidc  = rtrim( (string) ( getenv( 'DG_KEYCLOAK_INTERN' ) ?: getenv( 'DG_KEYCLOAK_URL' ) ), '/' ) . '/realms/' . ( getenv( 'DG_KEYCLOAK_REALM' ) ?: 'gemeenschap' ) . '/protocol/openid-connect';
	define( 'OIDC_LOGIN_TYPE', 'auto' );                 // wp-login.php gaat meteen naar Keycloak
	define( 'OIDC_CLIENT_ID', getenv( 'WP_CLIENT_ID' ) ?: 'wordpress' );
	define( 'OIDC_CLIENT_SECRET', (string) getenv( 'WP_CLIENT_SECRET' ) );
	define( 'OIDC_CLIENT_SCOPE', 'openid email profile organization' );
	// De browser gaat naar het openbare adres; tokens en gebruikersgegevens haalt WordPress intern op.
	define( 'OIDC_ENDPOINT_LOGIN_URL', $dg_realm . '/protocol/openid-connect/auth' );
	define( 'OIDC_ENDPOINT_LOGOUT_URL', $dg_realm . '/protocol/openid-connect/logout' );
	define( 'OIDC_ENDPOINT_TOKEN_URL', $dg_oidc . '/token' );
	define( 'OIDC_ENDPOINT_USERINFO_URL', $dg_oidc . '/userinfo' );
	define( 'OIDC_ENDPOINT_JWKS_URL', $dg_oidc . '/certs' );
	define( 'OIDC_ISSUER', $dg_realm );
	define( 'OIDC_CREATE_IF_DOES_NOT_EXIST', 1 );
	define( 'OIDC_LINK_EXISTING_USERS', 1 );
	define( 'OIDC_REDIRECT_USER_BACK', 1 );
	define( 'OIDC_REDIRECT_ON_LOGOUT', 1 );
	unset( $dg_realm, $dg_oidc );

	// WordPress weigert "veilige" verzoeken naar interne namen; Keycloak op het eigen netwerk mag wel.
	add_filter(
		'http_request_host_is_external',
		static function ( $extern, $host ) {
			$intern = wp_parse_url( (string) getenv( 'DG_KEYCLOAK_INTERN' ), PHP_URL_HOST );
			return $extern || ( $intern && $host === $intern );
		},
		10,
		2
	);
}

/** De rol in het netwerk, van zwaar naar licht. */
const DG_NETWERK = array(
	'kerngroep'    => 'Kerngroep',
	'members'      => 'Member',
	'contributors' => 'Contributor',
	'gebruikers'   => 'Gebruiker',
);
const DG_WERKVELDEN = array(
	'beheer'         => 'Beheer',
	'development'    => 'Development',
	'besluitvorming' => 'Besluitvorming',
	'anders'         => 'Anders',
);
const DG_PROFIELGROEP = 'Netwerk';
const DG_PROFIELVELDEN = array(
	'organisatie' => 'Organisatie',
	'functie'     => 'Functie',
	'werkveld'    => 'Werkveld',
);
const DG_MAILINGGROEP = 'Nieuwsbrief';

// ---------------------------------------------------------------------------
// Claims lezen
// ---------------------------------------------------------------------------

/** De groepen uit de claim, als paden ("/netwerk/members"). */
function dg_groepen( array $claim ): array {
	$g = $claim['groups'] ?? array();
	return array_values( array_filter( array_map( 'strval', is_array( $g ) ? $g : array( $g ) ) ) );
}

/** De zwaarste rol in het netwerk, als sleutel van DG_NETWERK. */
function dg_netwerkrol( array $groepen ): string {
	foreach ( array_keys( DG_NETWERK ) as $rol ) {
		if ( in_array( "/netwerk/$rol", $groepen, true ) ) {
			return $rol;
		}
	}
	return 'gebruikers';
}

/** De WordPress-rol: beheer, redactie of abonnee. */
function dg_wp_rol( array $groepen ): string {
	if ( in_array( '/site/beheer', $groepen, true ) ) {
		return 'administrator';
	}
	if ( in_array( '/site/redactie', $groepen, true ) ) {
		return 'editor';
	}
	return 'subscriber';
}

/** De bbPress-rol: beheer is keymaster, kerngroep en redactie modereren, de rest neemt deel. */
function dg_forumrol( array $groepen ): string {
	if ( in_array( '/site/beheer', $groepen, true ) ) {
		return 'bbp_keymaster';
	}
	if ( in_array( '/netwerk/kerngroep', $groepen, true ) || in_array( '/site/redactie', $groepen, true ) ) {
		return 'bbp_moderator';
	}
	return 'bbp_participant';
}

/**
 * De organisatie: de organisatie in Keycloak (Organizations) als die er is,
 * anders wat iemand zelf invulde. Keycloak geeft "organization" als lijst
 * aliassen, of als object met per alias de gegevens.
 */
function dg_organisatie( array $claim ): array {
	$org = $claim['organization'] ?? null;
	$alias = null;
	$naam  = null;
	if ( is_array( $org ) && $org ) {
		if ( array_is_list( $org ) ) {
			$alias = (string) $org[0];
		} else {
			$alias = (string) array_key_first( $org );
			$gegevens = $org[ $alias ];
			if ( is_array( $gegevens ) && ! empty( $gegevens['name'] ) ) {
				$naam = (string) $gegevens['name'];
			}
		}
	} elseif ( is_string( $org ) && '' !== $org ) {
		$alias = $org;
	}
	$opgegeven = trim( (string) ( $claim['organisatie'] ?? '' ) );
	return array(
		'alias' => $alias,
		'naam'  => $naam ?: ( $opgegeven ?: $alias ),
	);
}

function dg_werkveld( array $claim ): array {
	$w = $claim['werkveld'] ?? array();
	$w = is_array( $w ) ? $w : array( $w );
	return array_values( array_intersect( array_keys( DG_WERKVELDEN ), array_map( 'strval', $w ) ) );
}

function dg_toestemming( array $claim ): string {
	return 'ja' === ( $claim['toestemming_mailing'] ?? '' ) ? 'ja' : 'nee';
}

// ---------------------------------------------------------------------------
// Bij elke login: overnemen
// ---------------------------------------------------------------------------

add_action( 'openid-connect-generic-user-create', 'dg_neem_over', 10, 2 );
add_action( 'openid-connect-generic-update-user-using-current-claim', 'dg_neem_over', 10, 2 );

function dg_neem_over( $user, array $claim ): void {
	if ( ! $user instanceof WP_User ) {
		return;
	}
	$uid     = $user->ID;
	$groepen = dg_groepen( $claim );
	$org     = dg_organisatie( $claim );
	$werk    = dg_werkveld( $claim );

	// WordPress: naam en rol. De rol volgt Keycloak, ook bij een lagere rol.
	$naam = trim( ( $claim['given_name'] ?? '' ) . ' ' . ( $claim['family_name'] ?? '' ) );
	wp_update_user(
		array_filter(
			array(
				'ID'           => $uid,
				'first_name'   => $claim['given_name'] ?? null,
				'last_name'    => $claim['family_name'] ?? null,
				'display_name' => $naam ?: null,
				'nickname'     => $naam ?: null,
			),
			static fn( $w ) => null !== $w
		)
	);
	$rol = dg_wp_rol( $groepen );
	if ( ! in_array( $rol, (array) $user->roles, true ) || count( (array) $user->roles ) !== 1 ) {
		$user->set_role( $rol );
	}

	update_user_meta( $uid, 'dg_groepen', $groepen );
	update_user_meta( $uid, 'dg_netwerkrol', dg_netwerkrol( $groepen ) );
	update_user_meta( $uid, 'dg_organisatie', (string) $org['naam'] );
	update_user_meta( $uid, 'dg_organisatie_alias', (string) $org['alias'] );
	update_user_meta( $uid, 'dg_functie', trim( (string) ( $claim['functie'] ?? '' ) ) );
	update_user_meta( $uid, 'dg_werkveld', $werk );

	// Toestemming: elke wijziging met datum, ook hier (CiviCRM kan ontbreken).
	$nu    = dg_toestemming( $claim );
	$vorig = get_user_meta( $uid, 'dg_toestemming_mailing', true );
	if ( $vorig !== $nu ) {
		update_user_meta( $uid, 'dg_toestemming_mailing', $nu );
		$geschiedenis   = (array) get_user_meta( $uid, 'dg_toestemming_geschiedenis', true );
		$geschiedenis[] = array( 'datum' => gmdate( 'c' ), 'waarde' => $nu, 'bron' => 'keycloak' );
		update_user_meta( $uid, 'dg_toestemming_geschiedenis', array_values( array_filter( $geschiedenis ) ) );
	}

	dg_bbpress( $uid, $groepen );
	dg_buddypress( $uid, $groepen, $org, $claim, $werk );
	dg_civicrm( $uid, $groepen, $org, $nu, $vorig !== $nu );

	/** Voor wie meer wil koppelen: alles wat er net is overgenomen. */
	do_action( 'dg_gemeenschap_overgenomen', $uid, $claim, $groepen );
}

// ---------------------------------------------------------------------------
// bbPress
// ---------------------------------------------------------------------------

function dg_bbpress( int $uid, array $groepen ): void {
	if ( ! function_exists( 'bbp_set_user_role' ) ) {
		return;
	}
	$rol = dg_forumrol( $groepen );
	if ( function_exists( 'bbp_get_user_role' ) && bbp_get_user_role( $uid ) === $rol ) {
		return;
	}
	bbp_set_user_role( $uid, $rol );
}

// ---------------------------------------------------------------------------
// BuddyPress: ledentypen en profielvelden
// ---------------------------------------------------------------------------

add_action( 'bp_register_member_types', 'dg_ledentypen' );
function dg_ledentypen(): void {
	foreach ( DG_NETWERK as $sleutel => $naam ) {
		bp_register_member_type(
			$sleutel,
			array(
				'labels'        => array( 'name' => $naam . 's', 'singular_name' => $naam ),
				'has_directory' => $sleutel,
			)
		);
	}
}

/** De profielvelden in de groep "Netwerk"; maakt ze aan als ze er nog niet zijn. Geeft veld-id's per sleutel. */
function dg_profielvelden(): array {
	if ( ! function_exists( 'bp_is_active' ) || ! bp_is_active( 'xprofile' ) ) {
		return array();
	}
	$ids = get_option( 'dg_profielvelden' );
	if ( is_array( $ids ) && count( $ids ) === count( DG_PROFIELVELDEN ) && xprofile_get_field( reset( $ids ) ) ) {
		return $ids;
	}
	$groep = null;
	foreach ( (array) bp_xprofile_get_groups() as $g ) {
		if ( DG_PROFIELGROEP === $g->name ) {
			$groep = (int) $g->id;
		}
	}
	if ( ! $groep ) {
		$groep = (int) xprofile_insert_field_group(
			array(
				'name'        => DG_PROFIELGROEP,
				'description' => 'Uit je account bij Digitaal GemeenGoed. Wijzigen doe je daar (Mijn account).',
				'can_delete'  => false,
			)
		);
	}
	$ids = array();
	foreach ( DG_PROFIELVELDEN as $sleutel => $naam ) {
		$id = xprofile_get_field_id_from_name( $naam );
		if ( ! $id ) {
			$id = xprofile_insert_field(
				array(
					'field_group_id' => $groep,
					'name'           => $naam,
					'type'           => 'textbox',
					'can_delete'     => false,
					'is_required'    => false,
				)
			);
			// Alleen zichtbaar voor ingelogde leden.
			bp_xprofile_update_field_meta( $id, 'default_visibility', 'loggedin' );
			bp_xprofile_update_field_meta( $id, 'allow_custom_visibility', 'disabled' );
		}
		$ids[ $sleutel ] = (int) $id;
	}
	update_option( 'dg_profielvelden', $ids, false );
	return $ids;
}

function dg_buddypress( int $uid, array $groepen, array $org, array $claim, array $werk ): void {
	if ( function_exists( 'bp_set_member_type' ) ) {
		bp_set_member_type( $uid, dg_netwerkrol( $groepen ) );
	}
	$velden = dg_profielvelden();
	if ( ! $velden ) {
		return;
	}
	$waarden = array(
		'organisatie' => (string) $org['naam'],
		'functie'     => trim( (string) ( $claim['functie'] ?? '' ) ),
		'werkveld'    => implode( ', ', array_map( static fn( $w ) => DG_WERKVELDEN[ $w ], $werk ) ),
	);
	foreach ( $velden as $sleutel => $id ) {
		xprofile_set_field_data( $id, $uid, $waarden[ $sleutel ] );
	}
}

/** De velden uit Keycloak staan niet in het bewerkformulier van het profiel: wijzigen doe je in Keycloak. */
add_filter(
	'bp_after_has_profile_parse_args',
	static function ( $args ) {
		if ( function_exists( 'bp_is_user_profile_edit' ) && bp_is_user_profile_edit() ) {
			$ids = get_option( 'dg_profielvelden' );
			if ( is_array( $ids ) && $ids ) {
				$args['exclude_fields'] = implode( ',', array_merge( wp_parse_id_list( $args['exclude_fields'] ?? '' ), array_values( $ids ) ) );
			}
		}
		return $args;
	}
);

/** Leden, profielen en groepen alleen voor wie is ingelogd. Fora blijven leesbaar. */
add_action(
	'template_redirect',
	static function () {
		if ( is_user_logged_in() || ! function_exists( 'bp_is_members_component' ) ) {
			return;
		}
		if ( bp_is_members_component() || bp_is_user() || ( function_exists( 'bp_is_groups_component' ) && bp_is_groups_component() ) || ( function_exists( 'bp_is_activity_component' ) && bp_is_activity_component() ) ) {
			auth_redirect();
		}
	},
	1
);

// ---------------------------------------------------------------------------
// CiviCRM
// ---------------------------------------------------------------------------

function dg_civicrm( int $uid, array $groepen, array $org, string $toestemming, bool $toestemming_gewijzigd ): void {
	if ( ! function_exists( 'civi_wp' ) || ! function_exists( 'civicrm_initialize' ) || ! civicrm_initialize() || ! class_exists( '\Civi\Api4\Contact' ) ) {
		return;
	}
	try {
		$contact = dg_civicrm_contact( $uid );
		if ( ! $contact ) {
			return;
		}
		if ( $org['naam'] ) {
			dg_civicrm_werkgever( $contact, (string) $org['naam'] );
		}
		// Netwerkgroepen: lid van de eigen groep, uit de andere.
		$eigen = dg_netwerkrol( $groepen );
		foreach ( DG_NETWERK as $sleutel => $naam ) {
			dg_civicrm_lid( $contact, dg_civicrm_groep( "netwerk_$sleutel", "Netwerk: {$naam}s", false ), $sleutel === $eigen );
		}
		// Nieuwsbrief: volgt de toestemming in Keycloak; een wijziging als activiteit met datum.
		dg_civicrm_lid( $contact, dg_civicrm_groep( 'nieuwsbrief', DG_MAILINGGROEP, true ), 'ja' === $toestemming );
		if ( $toestemming_gewijzigd ) {
			\Civi\Api4\Activity::create( false )
				->addValue( 'activity_type_id:name', dg_civicrm_activiteitsoort() )
				->addValue( 'subject', 'ja' === $toestemming ? 'Toestemming nieuwsbrief: ja' : 'Toestemming nieuwsbrief: nee' )
				->addValue( 'details', 'Gegeven of gewijzigd in het account bij Digitaal GemeenGoed (Keycloak).' )
				->addValue( 'source_contact_id', $contact )
				->addValue( 'target_contact_id', array( $contact ) )
				->addValue( 'status_id:name', 'Completed' )
				->addValue( 'activity_date_time', gmdate( 'Y-m-d H:i:s' ) )
				->execute();
		}
	} catch ( \Throwable $e ) {
		error_log( 'dg-gemeenschap: CiviCRM bijwerken mislukt voor gebruiker ' . $uid . ': ' . $e->getMessage() );
	}
}

/** Het contact bij een WordPress-gebruiker; CiviCRM maakt het zelf bij het aanmaken van de gebruiker. */
function dg_civicrm_contact( int $uid ): ?int {
	$m = \Civi\Api4\UFMatch::get( false )->addWhere( 'uf_id', '=', $uid )->addSelect( 'contact_id' )->execute()->first();
	if ( ! $m ) {
		$user = get_userdata( $uid );
		if ( $user && class_exists( 'CRM_Core_BAO_UFMatch' ) ) {
			\CRM_Core_BAO_UFMatch::synchronize( $user, false, 'WordPress', 'Individual' );
			$m = \Civi\Api4\UFMatch::get( false )->addWhere( 'uf_id', '=', $uid )->addSelect( 'contact_id' )->execute()->first();
		}
	}
	return $m ? (int) $m['contact_id'] : null;
}

function dg_civicrm_werkgever( int $contact, string $organisatie ): void {
	$org = \Civi\Api4\Contact::get( false )
		->addWhere( 'contact_type', '=', 'Organization' )
		->addWhere( 'organization_name', '=', $organisatie )
		->addWhere( 'is_deleted', '=', false )
		->addSelect( 'id' )->execute()->first();
	$org_id = $org ? (int) $org['id'] : (int) \Civi\Api4\Contact::create( false )
		->addValue( 'contact_type', 'Organization' )
		->addValue( 'organization_name', $organisatie )
		->execute()->first()['id'];
	// employer_id zet ook de relatie "medewerker van" (Employee of).
	\Civi\Api4\Contact::update( false )->addWhere( 'id', '=', $contact )->addValue( 'employer_id', $org_id )->execute();
}

function dg_civicrm_groep( string $naam, string $titel, bool $mailinglijst ): int {
	$g = \Civi\Api4\Group::get( false )->addWhere( 'name', '=', $naam )->addSelect( 'id' )->execute()->first();
	if ( $g ) {
		return (int) $g['id'];
	}
	return (int) \Civi\Api4\Group::create( false )
		->addValue( 'name', $naam )
		->addValue( 'title', $titel )
		->addValue( 'description', $mailinglijst ? 'Wie toestemming gaf voor de nieuwsbrief en uitnodigingen (uit Keycloak).' : 'Rol in het netwerk, uit Keycloak. Niet hier wijzigen.' )
		->addValue( 'group_type:name', $mailinglijst ? array( 'Mailing List' ) : array( 'Access Control' ) )
		->addValue( 'is_reserved', true )
		->execute()->first()['id'];
}

function dg_civicrm_lid( int $contact, int $groep, bool $lid ): void {
	$huidig = \Civi\Api4\GroupContact::get( false )
		->addWhere( 'contact_id', '=', $contact )->addWhere( 'group_id', '=', $groep )
		->addSelect( 'id', 'status' )->execute()->first();
	$status = $lid ? 'Added' : 'Removed';
	if ( $huidig && $huidig['status'] === $status ) {
		return;
	}
	if ( ! $huidig && ! $lid ) {
		return;
	}
	\Civi\Api4\GroupContact::save( false )
		->addRecord( array( 'contact_id' => $contact, 'group_id' => $groep, 'status' => $status ) )
		->setMatch( array( 'contact_id', 'group_id' ) )
		->execute();
}

/** De activiteitsoort "Toestemming"; maakt hem aan als hij er nog niet is. */
function dg_civicrm_activiteitsoort(): string {
	$naam = 'dg_toestemming';
	$is   = \Civi\Api4\OptionValue::get( false )
		->addWhere( 'option_group_id:name', '=', 'activity_type' )->addWhere( 'name', '=', $naam )
		->execute()->first();
	if ( ! $is ) {
		\Civi\Api4\OptionValue::create( false )
			->addValue( 'option_group_id:name', 'activity_type' )
			->addValue( 'name', $naam )
			->addValue( 'label', 'Toestemming' )
			->addValue( 'description', 'Toestemming gegeven of ingetrokken (nieuwsbrief, uitnodigingen).' )
			->addValue( 'is_reserved', true )
			->execute();
	}
	return $naam;
}

// ---------------------------------------------------------------------------
// Aanmelden en inloggen alleen via Keycloak
// ---------------------------------------------------------------------------

/** De adressen van Keycloak, uit de omgeving (compose.yml). */
function dg_keycloak(): string {
	return rtrim( (string) getenv( 'DG_KEYCLOAK_URL' ), '/' );
}

/**
 * Geen Gravatar: dat zou bij elke ledenlijst een versleuteld e-mailadres van
 * elk lid naar een dienst buiten de EU sturen. Wie geen eigen foto heeft,
 * krijgt het standaardplaatje.
 */
add_filter( 'bp_core_fetch_avatar_no_grav', '__return_true' );
add_filter( 'pre_option_show_avatars', static fn() => '1' );
add_filter( 'pre_option_avatar_default', static fn() => 'blank' );
add_filter(
	'pre_get_avatar_data',
	static function ( $args ) {
		$args['url']          = function_exists( 'bp_core_avatar_default' ) ? bp_core_avatar_default( 'local', array( 'type' => 'full' ) ) : includes_url( 'images/blank.gif' );
		$args['found_avatar'] = true;
		return $args;
	}
);

/** Registreren in WordPress zelf staat uit; aanmelden gaat via Keycloak. */
add_filter( 'option_users_can_register', '__return_zero' );
add_filter( 'pre_option_users_can_register', '__return_zero' );

/**
 * /lid-worden/ (en de aanmeldpagina van BuddyPress) → het registratieformulier
 * van Keycloak, met prompt=create. Na het aanmelden kom je ingelogd terug.
 */
add_action(
	'template_redirect',
	static function () {
		$pad = trim( (string) wp_parse_url( $_SERVER['REQUEST_URI'] ?? '', PHP_URL_PATH ), '/' );
		$bp_aanmelden = function_exists( 'bp_is_register_page' ) && bp_is_register_page();
		if ( 'lid-worden' !== $pad && ! $bp_aanmelden ) {
			return;
		}
		if ( is_user_logged_in() ) {
			wp_safe_redirect( function_exists( 'bp_members_get_user_url' ) ? bp_members_get_user_url( get_current_user_id() ) : home_url( '/' ) );
			exit;
		}
		$url = do_shortcode( '[openid_connect_generic_auth_url]' );
		if ( ! $url || ! str_starts_with( $url, 'http' ) ) {
			wp_die( 'Aanmelden is nog niet ingesteld.' );
		}
		wp_redirect( add_query_arg( 'prompt', 'create', $url ) ); // phpcs:ignore WordPress.Security.SafeRedirect -- Keycloak, een ander domein.
		exit;
	},
	0
);

/** "Lid worden" en "Mijn account" in de menu's en widgets. */
add_shortcode( 'dg_lid_worden', static fn() => is_user_logged_in() ? '' : '<div class="wp-block-buttons is-layout-flex"><div class="wp-block-button"><a class="wp-block-button__link wp-element-button dg-lid-worden" href="' . esc_url( home_url( '/lid-worden/' ) ) . '">Lid worden</a></div></div>' );
add_shortcode(
	'dg_mijn_account',
	static fn() => dg_keycloak() ? '<a class="dg-mijn-account" href="' . esc_url( dg_keycloak() . '/realms/gemeenschap/account/' ) . '">Mijn account</a>' : ''
);

// ---------------------------------------------------------------------------
// Mail via de SMTP-dienst (dezelfde als Keycloak, CiviCRM en Redmine)
// ---------------------------------------------------------------------------

add_action(
	'phpmailer_init',
	static function ( $mailer ) {
		$host = getenv( 'SMTP_HOST' );
		if ( ! $host ) {
			return;
		}
		$mailer->isSMTP();
		$mailer->Host       = $host;
		$mailer->Port       = (int) ( getenv( 'SMTP_POORT' ) ?: 587 );
		$mailer->SMTPSecure = 465 === $mailer->Port ? 'ssl' : 'tls';
		$mailer->SMTPAuth   = (bool) getenv( 'SMTP_GEBRUIKER' );
		$mailer->Username   = (string) getenv( 'SMTP_GEBRUIKER' );
		$mailer->Password   = (string) getenv( 'SMTP_WACHTWOORD' );
		$afzender           = getenv( 'SMTP_AFZENDER' );
		if ( $afzender ) {
			$mailer->setFrom( $afzender, 'Digitaal GemeenGoed', false );
		}
	}
);

// ---------------------------------------------------------------------------
// Berichten namens Documenten (documenten.<domein>): bij het delen van een map
// of document krijgt de ontvanger een bericht in BuddyPress (en daarmee de
// gewone e-mailmelding). Alleen voor de dienst zelf: met het gedeelde geheim
// DG_BERICHT_GEHEIM, en alleen binnen het Docker-netwerk (Caddy stuurt
// /wp-json/dg/ niet door).
// ---------------------------------------------------------------------------

add_action(
	'rest_api_init',
	static function () {
		register_rest_route(
			'dg/v1',
			'/bericht',
			array(
				'methods'             => 'POST',
				'permission_callback' => 'dg_bericht_mag',
				'callback'            => 'dg_bericht',
			)
		);
	}
);

function dg_bericht_mag( WP_REST_Request $verzoek ): bool {
	$geheim = (string) getenv( 'DG_BERICHT_GEHEIM' );
	return '' !== $geheim && hash_equals( $geheim, (string) $verzoek->get_header( 'x-dg-sleutel' ) );
}

/**
 * Eén bericht per ontvanger (geen groepsgesprek). Ontvangers op e-mailadres
 * ("aan") en op groep uit Keycloak ("groepen", zoals /netwerk/members; die
 * staan bij elke login in de gebruikersmeta dg_groepen).
 */
function dg_bericht( WP_REST_Request $verzoek ) {
	if ( ! function_exists( 'messages_new_message' ) ) {
		return new WP_Error( 'dg_geen_berichten', 'De berichten van BuddyPress staan uit.', array( 'status' => 503 ) );
	}
	$van = get_user_by( 'email', sanitize_email( (string) $verzoek['van'] ) );
	if ( ! $van ) {
		return new WP_Error( 'dg_afzender', 'De afzender heeft geen account op de site.', array( 'status' => 422 ) );
	}
	$ontvangers = array();
	$onbekend   = array();
	foreach ( (array) $verzoek['aan'] as $adres ) {
		$u = get_user_by( 'email', sanitize_email( (string) $adres ) );
		if ( $u ) {
			$ontvangers[ $u->ID ] = true;
		} else {
			$onbekend[] = (string) $adres;
		}
	}
	foreach ( (array) $verzoek['groepen'] as $groep ) {
		$groep = (string) $groep;
		if ( ! preg_match( '#^(/[A-Za-z0-9 _.-]+)+$#', $groep ) ) {
			continue;
		}
		// dg_groepen is een geserialiseerde lijst; een pad staat er tussen aanhalingstekens in.
		$ids = get_users( array( 'meta_key' => 'dg_groepen', 'meta_value' => '"' . $groep . '"', 'meta_compare' => 'LIKE', 'fields' => 'ID', 'number' => 2000 ) );
		foreach ( $ids as $id ) {
			$ontvangers[ (int) $id ] = true;
		}
	}
	unset( $ontvangers[ $van->ID ] );
	$onderwerp = wp_strip_all_tags( (string) $verzoek['onderwerp'] );
	$inhoud    = wp_kses( (string) $verzoek['inhoud'], array( 'a' => array( 'href' => true ), 'strong' => array() ) );
	$verstuurd = 0;
	foreach ( array_keys( $ontvangers ) as $id ) {
		$draad = messages_new_message(
			array(
				'sender_id'  => $van->ID,
				'recipients' => array( $id ),
				'subject'    => $onderwerp,
				'content'    => $inhoud,
				'error_type' => 'wp_error',
			)
		);
		if ( $draad && ! is_wp_error( $draad ) ) {
			++$verstuurd;
		}
	}
	return array( 'verstuurd' => $verstuurd, 'onbekend' => $onbekend );
}

// ---------------------------------------------------------------------------
// Inrichten met wp-cli: wp dg inrichten
// ---------------------------------------------------------------------------

if ( defined( 'WP_CLI' ) && WP_CLI ) {
	/**
	 * De eerste inrichting: pagina's, fora, profielvelden, ledentypen. Opnieuw
	 * draaien kan zonder schade: wat er al is, blijft.
	 */
	WP_CLI::add_command(
		'dg inrichten',
		static function () {
			// De pagina's zelf komen uit inhoud/ in de repo (wp dg inhoud). De
			// voorbeelden van WordPress gaan weg, als niemand ze heeft gewijzigd.
			foreach ( array( array( 'sample-page', 'page' ), array( 'privacy-policy', 'page' ), array( 'hello-world', 'post' ) ) as [ $slug, $soort ] ) {
				$voorbeeld = get_page_by_path( $slug, OBJECT, $soort );
				if ( $voorbeeld && ! dg_inhoud_bron( $voorbeeld ) && $voorbeeld->post_modified_gmt === $voorbeeld->post_date_gmt ) {
					wp_delete_post( $voorbeeld->ID, true );
					WP_CLI::log( "Voorbeeld /$slug/ verwijderd." );
				}
			}
			// Logo en sitepictogram uit het thema (wordpress/thema/…/assets/).
			dg_thema_beeld( 'uil.png', static fn( $id ) => set_theme_mod( 'custom_logo', $id ) );
			dg_thema_beeld( 'icoon.png', static fn( $id ) => update_option( 'site_icon', $id ) );
			// De tabel voor toezeggingen (dg-crowdfunding.php).
			if ( function_exists( 'dg_cf_installeer' ) ) {
				dg_cf_installeer();
			}
			// Werkgroepen: BuddyPress-groepen, elk met een eigen forum onder "Werkgroepen".
			dg_werkgroepen();
			// De adressen van BuddyPress in het Nederlands.
			if ( function_exists( 'bp_core_get_directory_page_ids' ) ) {
				$adressen = array( 'members' => 'leden', 'groups' => 'groepen', 'activity' => 'activiteit' );
				foreach ( bp_core_get_directory_page_ids( 'all' ) as $onderdeel => $id ) {
					if ( isset( $adressen[ $onderdeel ] ) && get_post_field( 'post_name', $id ) !== $adressen[ $onderdeel ] ) {
						wp_update_post( array( 'ID' => $id, 'post_name' => $adressen[ $onderdeel ] ) );
						WP_CLI::log( "BuddyPress: /{$adressen[ $onderdeel ]}/." );
					}
				}
			}
			if ( function_exists( 'bbp_insert_forum' ) ) {
				$fora = array(
					'algemeen'         => array( 'Algemeen', 'Kennismaken, vragen, aankondigingen.' ),
					'office-assistent' => array( 'Office Assistent', 'Gebruik, ervaringen en ideeën voor de Office Assistent.' ),
					'beheer'           => array( 'Beheer', 'Installeren, bijwerken, koppelen en beheren.' ),
					'development'      => array( 'Development', 'Code, architectuur, standaarden en bijdragen.' ),
					'besluitvorming'   => array( 'Besluitvorming', 'Aanbesteden, invoeren, samenwerken tussen gemeenten.' ),
				);
				foreach ( $fora as $slug => [ $titel, $omschrijving ] ) {
					if ( get_page_by_path( $slug, OBJECT, 'forum' ) ) {
						WP_CLI::log( "Forum $titel bestaat al." );
						continue;
					}
					bbp_insert_forum( array( 'post_name' => $slug, 'post_title' => $titel, 'post_content' => $omschrijving ) );
					WP_CLI::log( "Forum $titel aangemaakt." );
				}
			}
			// De fora onder /forum/ (forum.<domein> verwijst daarheen).
			if ( function_exists( 'bbp_insert_forum' ) && 'forum' !== get_option( '_bbp_root_slug' ) ) {
				update_option( '_bbp_root_slug', 'forum' );
				flush_rewrite_rules( false );
				WP_CLI::log( 'Fora onder /forum/.' );
			}
			$velden = dg_profielvelden();
			WP_CLI::log( $velden ? 'Profielvelden: ' . implode( ', ', array_keys( $velden ) ) . '.' : 'BuddyPress (profielen) staat niet aan; profielvelden overgeslagen.' );
			WP_CLI::success( 'Ingericht.' );
		}
	);

	/**
	 * De werkgroepen. Een groep die al bestaat, blijft zoals beheerders hem
	 * hebben gemaakt; een groep die ontbreekt, komt erbij, met een forum.
	 */
	function dg_werkgroepen(): void {
		if ( ! function_exists( 'groups_create_group' ) || ! function_exists( 'bbp_insert_forum' ) ) {
			WP_CLI::log( 'Werkgroepen overgeslagen: BuddyPress-groepen of bbPress staan niet aan.' );
			return;
		}
		$groepen  = array(
			'common-ground'      => array( 'Common Ground', 'Architectuur, API\'s en herbruikbare bouwstenen.' ),
			'open-overheid'      => array( 'Open Overheid', 'Openbaarmaking, toegankelijkheid en publicatie.' ),
			'records-management' => array( 'Records Management', 'Metadata, archivering en duurzame toegankelijkheid.' ),
			'projectportfolio'   => array( 'Projectportfolio', 'Gezamenlijke prioriteiten en het starten van projecten.' ),
			'developers'         => array( 'Developers & contributors', 'Code, reviews en open samenwerking.' ),
			'implementatie'      => array( 'Implementatie & beheer', 'Van pilot naar dagelijkse dienstverlening.' ),
		);
		$beheer   = get_users( array( 'role' => 'administrator', 'number' => 1, 'orderby' => 'ID', 'fields' => 'ID' ) );
		$maker    = (int) ( $beheer[0] ?? 0 );
		$wortel   = get_page_by_path( 'werkgroepen', OBJECT, bbp_get_forum_post_type() );
		$wortelid = $wortel ? $wortel->ID : bbp_insert_forum(
			array(
				'post_name'    => 'werkgroepen',
				'post_title'   => 'Werkgroepen',
				'post_content' => 'De fora van de werkgroepen. Word lid van een werkgroep om mee te praten.',
			)
		);
		update_option( '_bbp_enable_group_forums', 1 );
		update_option( '_bbp_group_forums_root_id', $wortelid );
		foreach ( $groepen as $slug => [ $naam, $omschrijving ] ) {
			$gid = (int) groups_get_id( $slug );
			if ( ! $gid ) {
				$gid = (int) groups_create_group(
					array(
						'creator_id'   => $maker,
						'name'         => $naam,
						'slug'         => $slug,
						'description'  => $omschrijving,
						'status'       => 'public',
						'enable_forum' => 1,
					)
				);
				if ( $gid ) {
					WP_CLI::log( "Werkgroep $naam aangemaakt." );
				}
			}
			if ( ! $gid || ! empty( groups_get_groupmeta( $gid, 'forum_id' ) ) ) {
				continue;
			}
			$fid = bbp_insert_forum(
				array(
					'post_parent'  => $wortelid,
					'post_name'    => $slug,
					'post_title'   => $naam,
					'post_content' => $omschrijving,
				)
			);
			if ( $fid ) {
				bbp_add_forum_id_to_group( $gid, $fid );
				bbp_add_group_id_to_forum( $fid, $gid );
				WP_CLI::log( "Forum voor werkgroep $naam." );
			}
		}
	}

	/**
	 * Zet een afbeelding uit de map assets/ van het thema in de mediabibliotheek
	 * (alleen als die nieuw of veranderd is) en geeft het nummer door.
	 */
	function dg_thema_beeld( string $naam, callable $gebruik ): void {
		require_once ABSPATH . 'wp-admin/includes/file.php';
		require_once ABSPATH . 'wp-admin/includes/media.php';
		require_once ABSPATH . 'wp-admin/includes/image.php';
		$bestand = get_stylesheet_directory() . "/assets/$naam";
		if ( ! is_file( $bestand ) ) {
			return;
		}
		$hash  = hash_file( 'sha256', $bestand );
		$bron  = "thema/$naam";
		$oud   = get_posts( array( 'post_type' => 'attachment', 'post_status' => 'any', 'meta_key' => '_dg_bron', 'meta_value' => $bron, 'numberposts' => 1, 'fields' => 'ids' ) );
		$id    = $oud[0] ?? 0;
		if ( ! $id || get_post_meta( $id, '_dg_hash', true ) !== $hash ) {
			$tijdelijk = wp_tempnam( $naam );
			copy( $bestand, $tijdelijk );
			$nieuw = media_handle_sideload( array( 'name' => $naam, 'tmp_name' => $tijdelijk ), 0 );
			if ( is_wp_error( $nieuw ) ) {
				WP_CLI::warning( "$naam: " . $nieuw->get_error_message() );
				return;
			}
			update_post_meta( $nieuw, '_dg_bron', $bron );
			update_post_meta( $nieuw, '_dg_hash', $hash );
			if ( $id ) {
				wp_delete_attachment( $id, true );
			}
			$id = $nieuw;
			WP_CLI::log( "Thema: $naam." );
		}
		$gebruik( $id );
	}

	/** Laat zien wat er van iemand uit Keycloak is overgenomen: wp dg toon <gebruiker>. */
	WP_CLI::add_command(
		'dg toon',
		static function ( $args ) {
			$user = get_user_by( 'login', $args[0] ) ?: get_user_by( 'email', $args[0] );
			if ( ! $user ) {
				WP_CLI::error( 'Gebruiker niet gevonden.' );
			}
			$uit = array(
				'rol'         => implode( ',', $user->roles ),
				'netwerkrol'  => get_user_meta( $user->ID, 'dg_netwerkrol', true ),
				'ledentype'   => function_exists( 'bp_get_member_type' ) ? (string) bp_get_member_type( $user->ID ) : '',
				'forumrol'    => function_exists( 'bbp_get_user_role' ) ? (string) bbp_get_user_role( $user->ID ) : '',
				'organisatie' => get_user_meta( $user->ID, 'dg_organisatie', true ),
				'functie'     => get_user_meta( $user->ID, 'dg_functie', true ),
				'werkveld'    => implode( ',', (array) get_user_meta( $user->ID, 'dg_werkveld', true ) ),
				'toestemming' => get_user_meta( $user->ID, 'dg_toestemming_mailing', true ),
			);
			WP_CLI::log( (string) wp_json_encode( $uit, JSON_UNESCAPED_UNICODE ) );
		}
	);
}

// Een inlogverzoek dat WordPress niet meer kent (verlopen, of in een ander
// venster begonnen, zoals na de bevestigingsmail): één keer opnieuw beginnen.
// Wie al bij Keycloak is ingelogd, komt dan meteen terug en is ingelogd; lukt
// het weer niet, dan blijft de melding staan (het koekje voorkomt een lus).
add_action(
	'login_init',
	static function () {
		// phpcs:ignore WordPress.Security.NonceVerification.Recommended -- alleen een foutcode van de plugin.
		if ( 'invalid-state' !== ( $_GET['login-error'] ?? '' ) || is_user_logged_in() || ! empty( $_COOKIE['dg_oidc_opnieuw'] ) ) {
			return;
		}
		$url = do_shortcode( '[openid_connect_generic_auth_url]' );
		if ( ! is_string( $url ) || ! str_starts_with( $url, 'https://' ) ) {
			return;
		}
		setcookie( 'dg_oidc_opnieuw', '1', array( 'expires' => time() + 120, 'path' => '/', 'secure' => is_ssl(), 'httponly' => true, 'samesite' => 'Lax' ) );
		wp_redirect( $url ); // phpcs:ignore WordPress.Security.SafeRedirect.wp_redirect_wp_redirect -- naar Keycloak (OIDC_ENDPOINT_LOGIN_URL).
		exit;
	}
);
