<?php
/**
 * Plugin Name: Digitaal GemeenGoed — agenda
 * Description: De agenda op /agenda/: evenementen van de coöperatie, werkgroepen en projecten, en tips van leden voor evenementen elders. De formulieren staan op de site; de gegevens staan in CiviCRM (evenementen en deelnemers). Aanmelden met één klik, een wachtlijst, een iCal-abonnement per filter.
 *
 * Wie mag wat:
 *  - een evenement elders (tip, "extern") mag ieder lid toevoegen; het staat er meteen;
 *  - een eigen evenement namens de coöperatie: kerngroep, redactie, beheer;
 *    namens een werkgroep: wie de groep beheert of modereert (BuddyPress), en de kerngroep;
 *    namens een project: wie in Redmine het project beheert (Documenten weet dat:
 *    GET /intern/niveau), en de kerngroep;
 *  - bewerken: wie het toevoegde, wie namens de organisator mag, en de kerngroep.
 *
 * Per evenement: openbaar of alleen voor leden; aanmelden uit, alleen leden, of
 * iedereen (gasten met naam, e-mail en organisatie, na een bevestigingsmail);
 * een maximum met wachtlijst; de link naar de online meeting voor iedereen,
 * alleen voor leden, of alleen voor wie zich aanmeldde.
 *
 * In CiviCRM: het evenement (titel, samenvatting, omschrijving, begin, eind,
 * maximum, openbaar, adres via LocBlock) met de eigen velden in de groep
 * "dgg_evenement" (soort, tag, zichtbaar, aanmelden, links, auteur, afbeelding,
 * status), en elke aanmelding als deelnemer (Registered, On waitlist, Cancelled).
 * De pagina's van CiviCRM zelf voor evenementen (civicrm/event/…) zijn dicht voor
 * wie geen CiviCRM-beheerder is: alles loopt via /agenda/.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

const DG_AG_VERSIE = 3;          // ophogen als de adressen veranderen
const DG_AG_CIVI   = 1;          // ophogen als de velden in CiviCRM veranderen
const DG_AG_GROEP  = 'dgg_evenement';
const DG_AG_VELDEN = array(
	'soort'          => array( 'Soort (eigen of extern)', 'String' ),
	'tag'            => array( 'Organisator of onderwerp (tag)', 'String' ),
	'zichtbaar'      => array( 'Zichtbaar (openbaar of leden)', 'String' ),
	'aanmelden'      => array( 'Aanmelden (uit, leden, iedereen)', 'String' ),
	'online_link'    => array( 'Link naar de online meeting', 'String' ),
	'link_zichtbaar' => array( 'Link zichtbaar voor (openbaar, leden, deelnemers)', 'String' ),
	'extern_link'    => array( 'Link naar de organisator', 'String' ),
	'auteur'         => array( 'Toegevoegd door (WordPress-gebruiker)', 'Int' ),
	'afbeelding'     => array( 'Afbeelding (WordPress-media)', 'Int' ),
	'status'         => array( 'Status (gepland of afgelast)', 'String' ),
);
const DG_AG_TELT = array( 'Registered', 'Attended' );

// ---------------------------------------------------------------------------
// CiviCRM
// ---------------------------------------------------------------------------

function dg_ag_civi(): bool {
	static $ok = null;
	if ( null === $ok ) {
		$ok = function_exists( 'civicrm_initialize' ) && civicrm_initialize() && function_exists( 'civicrm_api4' );
		if ( $ok && (int) get_option( 'dg_ag_civi' ) !== DG_AG_CIVI ) {
			try {
				dg_ag_inrichten();
			} catch ( Throwable $e ) {
				error_log( 'dg-agenda: velden in CiviCRM: ' . $e->getMessage() );
			}
		}
	}
	return $ok;
}

/** APIv4 zonder de rechten van de ingelogde gebruiker (die controleert deze plugin zelf). */
function dg_ag_api( string $entiteit, string $actie, array $p = array() ): array {
	$p['checkPermissions'] = false;
	$uit                   = array();
	foreach ( civicrm_api4( $entiteit, $actie, $p ) as $rij ) {
		$uit[] = (array) $rij;
	}
	return $uit;
}

/** De eigen velden bij een evenement (groep dgg_evenement). */
function dg_ag_inrichten(): void {
	$groep = dg_ag_api( 'CustomGroup', 'get', array( 'where' => array( array( 'name', '=', DG_AG_GROEP ) ) ) );
	if ( ! $groep ) {
		$groep = dg_ag_api(
			'CustomGroup',
			'create',
			array(
				'values' => array(
					'name'        => DG_AG_GROEP,
					'title'       => 'Digitaal GemeenGoed',
					'extends'     => 'Event',
					'style'       => 'Inline',
					'is_active'   => true,
					'is_reserved' => false,
					'help_pre'    => 'Ingevuld door de agenda op de site (/agenda/). Hier liever niet wijzigen.',
				),
			)
		);
	}
	$gid = (int) $groep[0]['id'];
	$er  = array_column( dg_ag_api( 'CustomField', 'get', array( 'where' => array( array( 'custom_group_id', '=', $gid ) ), 'select' => array( 'name' ) ) ), 'name' );
	foreach ( DG_AG_VELDEN as $naam => [ $label, $type ] ) {
		if ( in_array( $naam, $er, true ) ) {
			continue;
		}
		dg_ag_api(
			'CustomField',
			'create',
			array(
				'values' => array(
					'custom_group_id' => $gid,
					'name'            => $naam,
					'label'           => $label,
					'data_type'       => $type,
					'html_type'       => 'Text',
					'is_active'       => true,
					'is_searchable'   => true,
				),
			)
		);
	}
	update_option( 'dg_ag_civi', DG_AG_CIVI );
}

function dg_ag_v( string $veld ): string {
	return DG_AG_GROEP . '.' . $veld;
}

/** Een tijd uit CiviCRM (plaatselijke tijd, zonder zone) als tijdstempel. */
function dg_ag_ts( ?string $s ): int {
	if ( ! $s ) {
		return 0;
	}
	try {
		return ( new DateTimeImmutable( $s, wp_timezone() ) )->getTimestamp();
	} catch ( Exception $e ) {
		return 0;
	}
}

function dg_ag_nu(): string {
	return wp_date( 'Y-m-d H:i:s' );
}

function dg_ag_select(): array {
	return array_merge(
		array( 'id', 'title', 'summary', 'description', 'start_date', 'end_date', 'max_participants', 'is_public', 'loc_block_id', 'loc_block_id.address_id', 'loc_block_id.address_id.street_address', 'loc_block_id.address_id.city' ),
		array_map( 'dg_ag_v', array_keys( DG_AG_VELDEN ) )
	);
}

/** Een rij uit CiviCRM als evenement. */
function dg_ag_maak( array $r ): array {
	$v = static fn( $k ) => $r[ dg_ag_v( $k ) ] ?? null;
	return array(
		'id'             => (int) $r['id'],
		'titel'          => (string) $r['title'],
		'samenvatting'   => (string) ( $r['summary'] ?? '' ),
		'omschrijving'   => (string) ( $r['description'] ?? '' ),
		'begin'          => dg_ag_ts( $r['start_date'] ?? '' ),
		'eind'           => dg_ag_ts( $r['end_date'] ?? '' ) ?: dg_ag_ts( $r['start_date'] ?? '' ) + 2 * HOUR_IN_SECONDS,
		'max'            => (int) ( $r['max_participants'] ?? 0 ),
		// Een evenement dat in CiviCRM zelf werd gemaakt: openbaar als het daar openbaar is.
		'openbaar'       => $v( 'zichtbaar' ) ? 'openbaar' === $v( 'zichtbaar' ) : ! empty( $r['is_public'] ),
		'soort'          => (string) ( $v( 'soort' ) ?: 'eigen' ),
		'tag'            => (string) ( $v( 'tag' ) ?: '' ),
		'aanmelden'      => (string) ( $v( 'aanmelden' ) ?: 'uit' ),
		'online_link'    => (string) ( $v( 'online_link' ) ?: '' ),
		'link_zichtbaar' => (string) ( $v( 'link_zichtbaar' ) ?: 'deelnemers' ),
		'extern_link'    => (string) ( $v( 'extern_link' ) ?: '' ),
		'auteur'         => (int) $v( 'auteur' ),
		'afbeelding'     => (int) $v( 'afbeelding' ),
		'afgelast'       => 'afgelast' === $v( 'status' ),
		'adres'          => (string) ( $r['loc_block_id.address_id.street_address'] ?? '' ),
		'plaats'         => (string) ( $r['loc_block_id.address_id.city'] ?? '' ),
		'loc_block'      => (int) ( $r['loc_block_id'] ?? 0 ),
		'adres_id'       => (int) ( $r['loc_block_id.address_id'] ?? 0 ),
	);
}

/**
 * Evenementen, eerstvolgende eerst. $f: tag, soort, mijn (bool), eerder (bool),
 * van/tot (Y-m-d H:i:s), limiet. Gasten zien alleen openbare.
 */
function dg_ag_evenementen( array $f = array() ): array {
	if ( ! dg_ag_civi() ) {
		return array();
	}
	$nu    = dg_ag_nu();
	$where = array( array( 'is_active', '=', true ) );
	if ( ! empty( $f['van'] ) ) {
		$where[] = array( 'end_date', '>=', $f['van'] );
		$where[] = array( 'start_date', '<=', $f['tot'] );
	} elseif ( ! empty( $f['eerder'] ) ) {
		$where[] = array( 'end_date', '<', $nu );
	} else {
		$where[] = array( 'end_date', '>=', $nu );
	}
	if ( ! is_user_logged_in() ) {
		$where[] = array( 'is_public', '=', true );
	}
	if ( ! empty( $f['tag'] ) ) {
		$where[] = array( dg_ag_v( 'tag' ), '=', (string) $f['tag'] );
	}
	if ( ! empty( $f['soort'] ) ) {
		$where[] = array( dg_ag_v( 'soort' ), '=', (string) $f['soort'] );
	}
	if ( ! empty( $f['mijn'] ) ) {
		$ids = array_keys( dg_ag_mijn_aanmeldingen() );
		if ( ! $ids ) {
			return array();
		}
		$where[] = array( 'id', 'IN', $ids );
	}
	try {
		$rijen = dg_ag_api(
			'Event',
			'get',
			array(
				'select'  => dg_ag_select(),
				'where'   => $where,
				'orderBy' => array( 'start_date' => empty( $f['eerder'] ) ? 'ASC' : 'DESC' ),
				'limit'   => (int) ( $f['limiet'] ?? 100 ),
			)
		);
	} catch ( Throwable $e ) {
		error_log( 'dg-agenda: evenementen ophalen: ' . $e->getMessage() );
		return array();
	}
	return array_map( 'dg_ag_maak', $rijen );
}

function dg_ag_evenement( int $id ): ?array {
	if ( ! $id || ! dg_ag_civi() ) {
		return null;
	}
	try {
		$r = dg_ag_api( 'Event', 'get', array( 'select' => dg_ag_select(), 'where' => array( array( 'id', '=', $id ), array( 'is_active', '=', true ) ) ) );
	} catch ( Throwable $e ) {
		return null;
	}
	return $r ? dg_ag_maak( $r[0] ) : null;
}

/** De deelnemers van evenementen: per evenement de rijen (contact, naam, organisatie, status). */
function dg_ag_deelnemers( array $ids, bool $namen = false ): array {
	$ids = array_values( array_filter( array_map( 'intval', $ids ) ) );
	if ( ! $ids || ! dg_ag_civi() ) {
		return array();
	}
	$select = array( 'id', 'event_id', 'contact_id', 'status_id:name', 'register_date' );
	if ( $namen ) {
		$select = array_merge( $select, array( 'contact_id.display_name', 'contact_id.employer_id.display_name' ) );
	}
	$uit = array_fill_keys( $ids, array() );
	try {
		foreach ( dg_ag_api( 'Participant', 'get', array( 'select' => $select, 'where' => array( array( 'event_id', 'IN', $ids ) ), 'orderBy' => array( 'register_date' => 'ASC' ) ) ) as $d ) {
			$uit[ (int) $d['event_id'] ][] = $d;
		}
	} catch ( Throwable $e ) {
		error_log( 'dg-agenda: deelnemers: ' . $e->getMessage() );
	}
	return $uit;
}

function dg_ag_aantal( array $rijen ): int {
	return count( array_filter( $rijen, static fn( $d ) => in_array( $d['status_id:name'] ?? '', DG_AG_TELT, true ) ) );
}

/** Het contact in CiviCRM bij een lid; maakt het zo nodig aan. */
function dg_ag_contact( int $uid ): int {
	$m = dg_ag_api( 'UFMatch', 'get', array( 'select' => array( 'contact_id' ), 'where' => array( array( 'uf_id', '=', $uid ) ) ) );
	if ( ! $m && class_exists( 'CRM_Core_BAO_UFMatch' ) && get_userdata( $uid ) ) {
		CRM_Core_BAO_UFMatch::synchronize( get_userdata( $uid ), false, 'WordPress', 'Individual' );
		$m = dg_ag_api( 'UFMatch', 'get', array( 'select' => array( 'contact_id' ), 'where' => array( array( 'uf_id', '=', $uid ) ) ) );
	}
	if ( $m ) {
		return (int) $m[0]['contact_id'];
	}
	$u = get_userdata( $uid );
	if ( ! $u ) {
		return 0;
	}
	$c = dg_ag_gast_contact( $u->user_email, $u->display_name, (string) get_user_meta( $uid, 'dg_organisatie', true ) );
	if ( $c ) {
		dg_ag_api( 'UFMatch', 'create', array( 'values' => array( 'uf_id' => $uid, 'contact_id' => $c, 'uf_name' => $u->user_email ) ) );
	}
	return $c;
}

/** Een contact op e-mailadres: het bestaande, of een nieuw (met de organisatie als werkgever). */
function dg_ag_gast_contact( string $email, string $naam, string $org ): int {
	$email = strtolower( trim( $email ) );
	$er    = dg_ag_api( 'Email', 'get', array( 'select' => array( 'contact_id' ), 'where' => array( array( 'email', '=', $email ), array( 'contact_id.is_deleted', '=', false ) ), 'limit' => 1 ) );
	if ( $er ) {
		return (int) $er[0]['contact_id'];
	}
	$delen = preg_split( '/\s+/', trim( $naam ), 2 );
	$c     = dg_ag_api(
		'Contact',
		'create',
		array(
			'values' => array(
				'contact_type' => 'Individual',
				'first_name'   => $delen[0] ?? '',
				'last_name'    => $delen[1] ?? '',
				'source'       => 'Agenda Digitaal GemeenGoed',
			),
		)
	);
	$cid   = (int) $c[0]['id'];
	dg_ag_api( 'Email', 'create', array( 'values' => array( 'contact_id' => $cid, 'email' => $email, 'is_primary' => true, 'location_type_id:name' => 'Work' ) ) );
	if ( '' !== trim( $org ) ) {
		$o   = dg_ag_api( 'Contact', 'get', array( 'select' => array( 'id' ), 'where' => array( array( 'contact_type', '=', 'Organization' ), array( 'organization_name', '=', trim( $org ) ), array( 'is_deleted', '=', false ) ), 'limit' => 1 ) );
		$oid = $o ? (int) $o[0]['id'] : (int) dg_ag_api( 'Contact', 'create', array( 'values' => array( 'contact_type' => 'Organization', 'organization_name' => trim( $org ) ) ) )[0]['id'];
		dg_ag_api( 'Contact', 'update', array( 'where' => array( array( 'id', '=', $cid ) ), 'values' => array( 'employer_id' => $oid ) ) );
	}
	return $cid;
}

/** De aanmeldingen van het ingelogde lid: evenement → status. */
function dg_ag_mijn_aanmeldingen(): array {
	static $mijn = null;
	if ( null !== $mijn ) {
		return $mijn;
	}
	$mijn = array();
	if ( ! is_user_logged_in() || ! dg_ag_civi() ) {
		return $mijn;
	}
	try {
		$c = dg_ag_api( 'UFMatch', 'get', array( 'select' => array( 'contact_id' ), 'where' => array( array( 'uf_id', '=', get_current_user_id() ) ) ) );
		if ( $c ) {
			foreach ( dg_ag_api( 'Participant', 'get', array( 'select' => array( 'event_id', 'status_id:name' ), 'where' => array( array( 'contact_id', '=', (int) $c[0]['contact_id'] ) ) ) ) as $d ) {
				$mijn[ (int) $d['event_id'] ] = (string) $d['status_id:name'];
			}
		}
	} catch ( Throwable $e ) {
		unset( $e );
	}
	return $mijn;
}

// ---------------------------------------------------------------------------
// Wie mag wat
// ---------------------------------------------------------------------------

function dg_ag_moderator( int $uid = 0 ): bool {
	if ( function_exists( 'dg_ber_moderator' ) ) {
		return dg_ber_moderator( $uid );
	}
	$uid = $uid ?: get_current_user_id();
	return $uid && ( user_can( $uid, 'edit_others_posts' ) || array_intersect( (array) get_user_meta( $uid, 'dg_groepen', true ), array( '/netwerk/kerngroep', '/site/redactie', '/site/beheer' ) ) );
}

/** Het niveau van een lid in een ruimte van Documenten (0 = geen lid, 3 = beheren). */
function dg_ag_documenten_niveau( string $ruimte, string $email ): int {
	$basis  = rtrim( (string) getenv( 'DG_DOCUMENTEN_INTERN' ), '/' );
	$geheim = (string) getenv( 'DG_BERICHT_GEHEIM' );
	if ( ! $basis || ! $geheim ) {
		return 0;
	}
	$sleutel = 'dg_ag_niv_' . md5( $ruimte . '|' . strtolower( $email ) );
	$n       = get_transient( $sleutel );
	if ( false !== $n ) {
		return (int) $n;
	}
	$r = wp_remote_get(
		$basis . '/intern/niveau?' . http_build_query( array( 'ruimte' => $ruimte, 'email' => $email ) ),
		array( 'timeout' => 5, 'headers' => array( 'X-DG-Sleutel' => $geheim ) )
	);
	$n = 200 === (int) wp_remote_retrieve_response_code( $r ) ? (int) ( json_decode( wp_remote_retrieve_body( $r ), true )['niveau'] ?? 0 ) : 0;
	set_transient( $sleutel, $n, 10 * MINUTE_IN_SECONDS );
	return $n;
}

// Documenten op het interne netwerk mag WordPress opvragen.
add_filter(
	'http_request_host_is_external',
	static function ( $extern, $host ) {
		$intern = wp_parse_url( (string) getenv( 'DG_DOCUMENTEN_INTERN' ), PHP_URL_HOST );
		return $extern || ( $intern && $host === $intern );
	},
	10,
	2
);

/** Mag dit lid een eigen evenement plaatsen namens deze tag (cooperatie, werkgroep-…, project-…)? */
function dg_ag_mag_namens( string $tag, int $uid = 0 ): bool {
	$uid = $uid ?: get_current_user_id();
	if ( ! $uid || '' === $tag ) {
		return false;
	}
	if ( dg_ag_moderator( $uid ) ) {
		return true;
	}
	if ( str_starts_with( $tag, 'werkgroep-' ) && function_exists( 'groups_get_id' ) ) {
		$gid = (int) groups_get_id( substr( $tag, 10 ) );
		return $gid && ( groups_is_user_admin( $uid, $gid ) || groups_is_user_mod( $uid, $gid ) );
	}
	if ( str_starts_with( $tag, 'project-' ) ) {
		$u = get_userdata( $uid );
		return $u && dg_ag_documenten_niveau( substr( $tag, 8 ), $u->user_email ) >= 3;
	}
	return false;
}

function dg_ag_mag_bewerken( array $e, int $uid = 0 ): bool {
	$uid = $uid ?: get_current_user_id();
	if ( ! $uid ) {
		return false;
	}
	return $e['auteur'] === $uid || dg_ag_moderator( $uid ) || ( 'eigen' === $e['soort'] && dg_ag_mag_namens( $e['tag'], $uid ) );
}

/** Mag de bezoeker dit evenement zien? */
function dg_ag_mag_zien( array $e ): bool {
	return $e['openbaar'] || is_user_logged_in();
}

/** De vaste tags (coöperatie, werkgroepen, projecten), uit het nieuws (dg-berichten.php). */
function dg_ag_vaste_tags(): array {
	if ( ! defined( 'DG_BER_TAG' ) ) {
		return array( 'cooperatie' => 'Coöperatie' );
	}
	$uit = array();
	foreach ( (array) get_terms( array( 'taxonomy' => DG_BER_TAG, 'hide_empty' => false, 'meta_key' => 'dg_soort', 'number' => 300 ) ) as $t ) { // phpcs:ignore WordPress.DB.SlowDBQuery
		if ( $t instanceof WP_Term ) {
			$uit[ $t->slug ] = function_exists( 'dg_ber_tagnaam' ) ? dg_ber_tagnaam( $t ) : $t->name;
		}
	}
	uksort( $uit, static fn( $a, $b ) => ( 'cooperatie' === $b ) <=> ( 'cooperatie' === $a ) ?: strcmp( $a, $b ) );
	return $uit;
}

function dg_ag_tagnaam( string $tag ): string {
	return dg_ag_vaste_tags()[ $tag ] ?? $tag;
}

function dg_ag_tagsoort( string $tag ): string {
	return 'cooperatie' === $tag ? 'Coöperatie' : ( str_starts_with( $tag, 'werkgroep-' ) ? 'Werkgroep' : ( str_starts_with( $tag, 'project-' ) ? 'Project' : '' ) );
}

// ---------------------------------------------------------------------------
// Adressen
// ---------------------------------------------------------------------------

add_action(
	'init',
	static function () {
		$id = '([0-9]+)(?:-[^/]*)?';
		add_rewrite_rule( '^agenda/?$', 'index.php?dg_agenda=lijst', 'top' );
		add_rewrite_rule( '^agenda/nieuw/?$', 'index.php?dg_agenda=nieuw', 'top' );
		add_rewrite_rule( '^agenda/ical/?$', 'index.php?dg_agenda=ical', 'top' );
		add_rewrite_rule( '^agenda/ical/([a-f0-9]{32})/?$', 'index.php?dg_agenda=ical&dg_ag_sleutel=$matches[1]', 'top' );
		add_rewrite_rule( '^agenda/bevestig/([a-f0-9]{48})/?$', 'index.php?dg_agenda=bevestig&dg_ag_sleutel=$matches[1]', 'top' );
		add_rewrite_rule( '^agenda/afmelden/([0-9]+)-([a-f0-9]{24})/?$', 'index.php?dg_agenda=gastafmelden&dg_ag_id=$matches[1]&dg_ag_sleutel=$matches[2]', 'top' );
		add_rewrite_rule( "^agenda/$id/bewerken/?$", 'index.php?dg_agenda=bewerken&dg_ag_id=$matches[1]', 'top' );
		add_rewrite_rule( "^agenda/$id/ics/?$", 'index.php?dg_agenda=ics&dg_ag_id=$matches[1]', 'top' );
		add_rewrite_rule( "^agenda/$id/deelnemers\\.csv/?$", 'index.php?dg_agenda=csv&dg_ag_id=$matches[1]', 'top' );
		add_rewrite_rule( "^agenda/$id/?$", 'index.php?dg_agenda=evenement&dg_ag_id=$matches[1]', 'top' );
		$regels = get_option( 'rewrite_rules' );
		if ( (int) get_option( 'dg_ag_versie' ) !== DG_AG_VERSIE || ( is_array( $regels ) && ! isset( $regels['^agenda/?$'] ) ) ) {
			update_option( 'dg_ag_versie', DG_AG_VERSIE );
			add_action( 'wp_loaded', static fn() => flush_rewrite_rules( false ) );
		}
	}
);

add_filter( 'query_vars', static fn( $v ) => array_merge( $v, array( 'dg_agenda', 'dg_ag_id', 'dg_ag_sleutel' ) ) );
add_filter( 'pre_handle_404', static fn( $p ) => get_query_var( 'dg_agenda' ) ? true : $p );

function dg_ag_url( ?array $e = null, string $na = '' ): string {
	if ( ! $e ) {
		return home_url( '/agenda/' . ( $na ? trailingslashit( $na ) : '' ) );
	}
	return home_url( '/agenda/' . $e['id'] . '-' . sanitize_title( $e['titel'] ) . '/' . ( $na ? trailingslashit( $na ) : '' ) );
}

// De pagina's van CiviCRM voor evenementen: alleen voor wie CiviCRM beheert; de rest gaat naar de agenda.
add_action(
	'parse_request',
	static function () {
		$pad = (string) ( $_SERVER['REQUEST_URI'] ?? '' ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
		$q   = (string) ( $_GET['q'] ?? '' ); // phpcs:ignore WordPress.Security.NonceVerification, WordPress.Security.ValidatedSanitizedInput
		if ( ! str_contains( $pad, 'civicrm/event' ) && ! str_starts_with( $q, 'civicrm/event' ) ) {
			return;
		}
		if ( is_admin() || current_user_can( 'manage_options' ) || current_user_can( 'edit_all_events' ) || current_user_can( 'access_civievent' ) ) {
			return;
		}
		wp_safe_redirect( home_url( '/agenda/' ) );
		exit;
	},
	1
);

// ---------------------------------------------------------------------------
// Opslaan
// ---------------------------------------------------------------------------

function dg_ag_melding_zet( array $regels, bool $fout = false ): void {
	if ( function_exists( 'dg_ber_melding_zet' ) ) {
		dg_ber_melding_zet( $regels, $fout );
		return;
	}
	set_transient( 'dg_ber_melding_' . get_current_user_id(), array( $regels, $fout ), 10 * MINUTE_IN_SECONDS );
}

function dg_ag_terug( string $url, array $regels, bool $fout = false ): void {
	dg_ag_melding_zet( $regels, $fout );
	wp_safe_redirect( $url );
	exit;
}

add_action( 'admin_post_nopriv_dg_evenement_opslaan', static fn() => auth_redirect() );
add_action(
	'admin_post_dg_evenement_opslaan',
	static function () {
		$uid = get_current_user_id();
		if ( ! $uid || ! check_admin_referer( 'dg_evenement_opslaan' ) ) {
			wp_die( 'Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.', 'Verlopen', array( 'response' => 403 ) );
		}
		if ( ! dg_ag_civi() ) {
			dg_ag_terug( dg_ag_url(), array( 'De agenda is nu niet bereikbaar (CiviCRM). Probeer het later opnieuw.' ), true );
		}
		$id     = absint( $_POST['id'] ?? 0 );
		$oud    = $id ? dg_ag_evenement( $id ) : null;
		$terug  = $oud ? dg_ag_url( $oud, 'bewerken' ) : dg_ag_url( null, 'nieuw' );
		if ( $id && ( ! $oud || ! dg_ag_mag_bewerken( $oud ) ) ) {
			wp_die( 'Dit evenement kun je niet bewerken.', 'Niet toegestaan', array( 'response' => 403 ) );
		}
		$t = static fn( $k, $max = 200 ) => mb_substr( trim( sanitize_text_field( wp_unslash( $_POST[ $k ] ?? '' ) ) ), 0, $max );
		$soort  = 'extern' === $t( 'soort' ) ? 'extern' : 'eigen';
		$tag    = sanitize_title( 'extern' === $soort ? $t( 'tag_extern' ) : $t( 'tag' ) );
		$titel  = $t( 'titel', 200 );
		$samen  = $t( 'samenvatting', 300 );
		$omschr = mb_substr( trim( sanitize_textarea_field( wp_unslash( $_POST['omschrijving'] ?? '' ) ) ), 0, 5000 );
		$datum  = $t( 'datum', 10 );
		$van    = $t( 'van', 5 );
		$einddt = $t( 'einddatum', 10 ) ?: $datum;
		$tot    = $t( 'tot', 5 );
		$adres  = $t( 'adres' );
		$plaats = $t( 'plaats', 100 );
		$online = esc_url_raw( $t( 'online_link', 500 ), array( 'https', 'http' ) );
		$extern = esc_url_raw( $t( 'extern_link', 500 ), array( 'https', 'http' ) );
		$fouten = array();
		if ( '' === $titel ) {
			$fouten[] = 'Vul een titel in.';
		}
		$begin = preg_match( '/^\d{4}-\d{2}-\d{2}$/', $datum ) && preg_match( '/^\d{2}:\d{2}$/', $van ) ? "$datum $van:00" : '';
		if ( ! $begin ) {
			$fouten[] = 'Vul de datum en de begintijd in.';
		}
		$eind = preg_match( '/^\d{4}-\d{2}-\d{2}$/', $einddt ) && preg_match( '/^\d{2}:\d{2}$/', $tot ) ? "$einddt $tot:00" : ( $begin ? wp_date( 'Y-m-d H:i:s', dg_ag_ts( $begin ) + 2 * HOUR_IN_SECONDS ) : '' );
		if ( $begin && $eind && dg_ag_ts( $eind ) <= dg_ag_ts( $begin ) ) {
			$fouten[] = 'Het einde ligt voor het begin.';
		}
		if ( 'extern' === $soort && ! $extern ) {
			$fouten[] = 'Geef bij een evenement elders de link naar de organisator.';
		}
		if ( 'eigen' === $soort && ( '' === $tag || ! dg_ag_mag_namens( $tag ) ) && ! ( $oud && $oud['tag'] === $tag && dg_ag_mag_bewerken( $oud ) ) ) {
			$fouten[] = 'Kies namens wie je het organiseert: de coöperatie, een werkgroep of een project dat jij beheert. Iets elders? Kies dan "Een evenement elders".';
		}
		if ( 'extern' === $soort && '' !== $tag && ! isset( dg_ag_vaste_tags()[ $tag ] ) ) {
			$tag = '';
		}
		if ( $fouten ) {
			set_transient( 'dg_ag_invoer_' . $uid, wp_unslash( $_POST ), 10 * MINUTE_IN_SECONDS );
			dg_ag_terug( $terug, $fouten, true );
		}
		$aanmelden = 'eigen' === $soort ? ( in_array( $t( 'aanmelden' ), array( 'leden', 'iedereen' ), true ) ? $t( 'aanmelden' ) : 'uit' ) : 'uit';
		$openbaar  = 'openbaar' === $t( 'zichtbaar' );
		// Gasten aanmelden bij een evenement dat alleen voor leden is: dat gaat niet samen.
		if ( ! $openbaar && 'iedereen' === $aanmelden ) {
			$aanmelden = 'leden';
		}
		$linkzicht = in_array( $t( 'link_zichtbaar' ), array( 'openbaar', 'leden', 'deelnemers' ), true ) ? $t( 'link_zichtbaar' ) : 'deelnemers';
		if ( 'uit' === $aanmelden && 'deelnemers' === $linkzicht ) {
			$linkzicht = 'leden';
		}
		$waarden = array(
			'title'                  => $titel,
			'summary'                => $samen,
			'description'            => wpautop( esc_html( $omschr ) ),
			'start_date'             => $begin,
			'end_date'               => $eind,
			'is_public'              => $openbaar,
			'is_active'              => true,
			'is_online_registration' => false,
			'is_show_location'       => true,
			'max_participants'       => 'uit' === $aanmelden ? null : ( absint( $_POST['max'] ?? 0 ) ?: null ),
			'has_waitlist'           => true,
			dg_ag_v( 'soort' )          => $soort,
			dg_ag_v( 'tag' )            => $tag,
			dg_ag_v( 'zichtbaar' )      => $openbaar ? 'openbaar' : 'leden',
			dg_ag_v( 'aanmelden' )      => $aanmelden,
			dg_ag_v( 'online_link' )    => $online,
			dg_ag_v( 'link_zichtbaar' ) => $linkzicht,
			dg_ag_v( 'extern_link' )    => $extern,
		);
		try {
			// Het adres (LocBlock met Address), alleen als er een is.
			if ( '' !== $adres || '' !== $plaats ) {
				$a = array( 'street_address' => $adres, 'city' => $plaats, 'location_type_id:name' => 'Main', 'country_id:name' => 'NL' );
				if ( $oud && $oud['adres_id'] ) {
					dg_ag_api( 'Address', 'update', array( 'where' => array( array( 'id', '=', $oud['adres_id'] ) ), 'values' => $a ) );
				} else {
					$aid                         = (int) dg_ag_api( 'Address', 'create', array( 'values' => $a ) )[0]['id'];
					$waarden['loc_block_id']     = (int) dg_ag_api( 'LocBlock', 'create', array( 'values' => array( 'address_id' => $aid ) ) )[0]['id'];
				}
			} elseif ( $oud && $oud['loc_block'] ) {
				$waarden['loc_block_id'] = null;
			}
			if ( $oud ) {
				dg_ag_api( 'Event', 'update', array( 'where' => array( array( 'id', '=', $oud['id'] ) ), 'values' => $waarden ) );
				$eid = $oud['id'];
			} else {
				$waarden['event_type_id:name'] = 'Meeting';
				$waarden[ dg_ag_v( 'auteur' ) ]  = $uid;
				$waarden[ dg_ag_v( 'status' ) ]  = 'gepland';
				$waarden['created_id']         = dg_ag_contact( $uid ) ?: null;
				$eid                           = (int) dg_ag_api( 'Event', 'create', array( 'values' => $waarden ) )[0]['id'];
			}
		} catch ( Throwable $e ) {
			error_log( 'dg-agenda: opslaan: ' . $e->getMessage() );
			set_transient( 'dg_ag_invoer_' . $uid, wp_unslash( $_POST ), 10 * MINUTE_IN_SECONDS );
			dg_ag_terug( $terug, array( 'Het evenement kon niet worden opgeslagen. Probeer het opnieuw; lukt het niet, meld het dan.' ), true );
		}
		// De afbeelding: in de mediabibliotheek van de site (zonder EXIF, net als bij het nieuws).
		$meldingen = array( $oud ? 'Evenement bijgewerkt.' : 'Het evenement staat in de agenda.' );
		$f         = $_FILES['afbeelding'] ?? null; // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
		if ( is_array( $f ) && UPLOAD_ERR_OK === (int) $f['error'] && function_exists( 'dg_ber_bewaar_beeld' ) ) {
			$beeld = (int) $f['size'] <= 12 * MB_IN_BYTES ? dg_ber_bewaar_beeld( (string) $f['tmp_name'], sanitize_file_name( (string) $f['name'] ), 0, 1600 ) : new WP_Error( 'groot', 'te groot' );
			if ( is_wp_error( $beeld ) ) {
				$meldingen[] = 'De afbeelding is niet opgeslagen: ' . $beeld->get_error_message();
			} else {
				if ( $oud && $oud['afbeelding'] ) {
					wp_delete_attachment( $oud['afbeelding'], true );
				}
				dg_ag_api( 'Event', 'update', array( 'where' => array( array( 'id', '=', $eid ) ), 'values' => array( dg_ag_v( 'afbeelding' ) => (int) $beeld ) ) );
			}
		}
		$e = dg_ag_evenement( $eid );
		if ( ! $oud && $e && ! empty( $_POST['nieuws'] ) ) {
			dg_ag_nieuwsbericht( $e );
			$meldingen[] = 'Er staat ook een bericht over in het nieuws.';
		}
		delete_transient( 'dg_ag_invoer_' . $uid );
		dg_ag_terug( $e ? dg_ag_url( $e ) : dg_ag_url(), $meldingen );
	}
);

/** Een bericht in het nieuws over een nieuw evenement (dg-berichten.php), met een kaart naar de agenda. */
function dg_ag_nieuwsbericht( array $e ): void {
	if ( ! defined( 'DG_BER_TYPE' ) ) {
		return;
	}
	$tekst = ( $e['samenvatting'] ? $e['samenvatting'] . "\n\n" : '' ) . dg_ag_wanneer( $e ) . ( dg_ag_waar( $e, false ) ? ', ' . dg_ag_waar( $e, false ) : '' ) . '.';
	$pid   = wp_insert_post(
		wp_slash(
			array(
				'post_type'      => DG_BER_TYPE,
				'post_status'    => 'publish',
				'post_title'     => $e['titel'],
				'post_content'   => $tekst,
				'post_author'    => get_current_user_id(),
				'comment_status' => 'open',
			)
		)
	);
	if ( ! $pid || is_wp_error( $pid ) ) {
		return;
	}
	update_post_meta( $pid, '_dg_zichtbaar', $e['openbaar'] ? 'openbaar' : 'leden' );
	update_post_meta(
		$pid,
		'_dg_link',
		array(
			'url'   => dg_ag_url( $e ),
			'titel' => $e['titel'],
			'tekst' => dg_ag_wanneer( $e ) . ( 'uit' !== $e['aanmelden'] ? ' · aanmelden via de agenda' : '' ),
			'site'  => 'Agenda',
			'beeld' => $e['afbeelding'],
			'eigen' => 1,
		)
	);
	if ( $e['afbeelding'] ) {
		set_post_thumbnail( $pid, $e['afbeelding'] );
	}
	if ( $e['tag'] ) {
		$term = get_term_by( 'slug', $e['tag'], DG_BER_TAG );
		if ( $term ) {
			wp_set_object_terms( $pid, array( (int) $term->term_id ), DG_BER_TAG );
		}
	}
	if ( function_exists( 'dg_ber_activiteit' ) ) {
		dg_ber_activiteit( get_post( $pid ), false );
	}
}

add_action(
	'admin_post_dg_evenement_beheer',
	static function () {
		$id = absint( $_POST['id'] ?? 0 );
		$e  = dg_ag_evenement( $id );
		if ( ! $e || ! check_admin_referer( 'dg_evenement_beheer_' . $id ) || ! dg_ag_mag_bewerken( $e ) ) {
			wp_die( 'Dat lukte niet.', 'Fout', array( 'response' => 403 ) );
		}
		$wat        = sanitize_key( wp_unslash( $_POST['wat'] ?? '' ) );
		$deelnemers = dg_ag_deelnemers( array( $id ) )[ $id ] ?? array();
		if ( 'verwijderen' === $wat ) {
			if ( dg_ag_aantal( $deelnemers ) ) {
				dg_ag_terug( dg_ag_url( $e ), array( 'Er zijn aanmeldingen: gelast het evenement af in plaats van het te verwijderen. Dan krijgen de deelnemers een bericht.' ), true );
			}
			dg_ag_api( 'Event', 'delete', array( 'where' => array( array( 'id', '=', $id ) ) ) );
			if ( $e['afbeelding'] ) {
				wp_delete_attachment( $e['afbeelding'], true );
			}
			dg_ag_terug( dg_ag_url(), array( 'Het evenement is verwijderd.' ) );
		}
		if ( 'afgelasten' === $wat || 'doorgaan' === $wat ) {
			dg_ag_api( 'Event', 'update', array( 'where' => array( array( 'id', '=', $id ) ), 'values' => array( dg_ag_v( 'status' ) => 'afgelasten' === $wat ? 'afgelast' : 'gepland' ) ) );
			$n = 0;
			if ( 'afgelasten' === $wat ) {
				$reden = mb_substr( trim( sanitize_textarea_field( wp_unslash( $_POST['reden'] ?? '' ) ) ), 0, 1000 );
				foreach ( dg_ag_deelnemers( array( $id ), true )[ $id ] ?? array() as $d ) {
					if ( in_array( $d['status_id:name'], array( 'Registered', 'On waitlist' ), true ) && dg_ag_mail_contact( (int) $d['contact_id'], 'Afgelast: ' . $e['titel'], "Het evenement \"{$e['titel']}\" (" . dg_ag_wanneer( $e ) . ") gaat niet door.\n\n" . ( $reden ? "$reden\n\n" : '' ) . dg_ag_url( $e ) . "\n" ) ) {
						++$n;
					}
				}
			}
			dg_ag_terug( dg_ag_url( $e ), array( 'afgelasten' === $wat ? "Afgelast. $n deelnemers kregen een mail." : 'Het evenement gaat weer door.' ) );
		}
		wp_die( 'Onbekende opdracht.', 'Fout', array( 'response' => 400 ) );
	}
);

// ---------------------------------------------------------------------------
// Aanmelden en afmelden
// ---------------------------------------------------------------------------

/** Kan er nu worden aangemeld? Geeft een reden als het niet kan. */
function dg_ag_aanmelden_kan( array $e ): string {
	if ( 'uit' === $e['aanmelden'] ) {
		return 'Aanmelden via de site staat uit.';
	}
	if ( $e['afgelast'] ) {
		return 'Dit evenement is afgelast.';
	}
	if ( $e['begin'] < time() ) {
		return 'Dit evenement is al begonnen.';
	}
	return '';
}

/**
 * Een aanmelding (nieuw, of een eerdere afmelding weer aan): Registered, of
 * On waitlist als het vol is. Geeft de status.
 */
function dg_ag_aanmelding( array $e, int $contact ): string {
	$rijen  = dg_ag_deelnemers( array( $e['id'] ) )[ $e['id'] ] ?? array();
	$status = $e['max'] && dg_ag_aantal( $rijen ) >= $e['max'] ? 'On waitlist' : 'Registered';
	foreach ( $rijen as $d ) {
		if ( (int) $d['contact_id'] === $contact ) {
			if ( in_array( $d['status_id:name'], array( 'Registered', 'Attended', 'On waitlist' ), true ) ) {
				return (string) $d['status_id:name'];
			}
			dg_ag_api( 'Participant', 'update', array( 'where' => array( array( 'id', '=', (int) $d['id'] ) ), 'values' => array( 'status_id:name' => $status, 'register_date' => dg_ag_nu() ) ) );
			return $status;
		}
	}
	dg_ag_api(
		'Participant',
		'create',
		array(
			'values' => array(
				'event_id'       => $e['id'],
				'contact_id'     => $contact,
				'status_id:name' => $status,
				'role_id:name'   => array( 'Attendee' ),
				'register_date'  => dg_ag_nu(),
				'source'         => 'Agenda Digitaal GemeenGoed',
			),
		)
	);
	return $status;
}

/** Afmelden; de eerste op de wachtlijst schuift door (en krijgt een mail). */
function dg_ag_afmelding( array $e, int $contact ): bool {
	$rijen = dg_ag_deelnemers( array( $e['id'] ) )[ $e['id'] ] ?? array();
	$weg   = false;
	foreach ( $rijen as $d ) {
		if ( (int) $d['contact_id'] === $contact && 'Cancelled' !== $d['status_id:name'] ) {
			$telde = in_array( $d['status_id:name'], DG_AG_TELT, true );
			dg_ag_api( 'Participant', 'update', array( 'where' => array( array( 'id', '=', (int) $d['id'] ) ), 'values' => array( 'status_id:name' => 'Cancelled' ) ) );
			$weg = true;
			if ( $telde && $e['begin'] > time() ) {
				foreach ( $rijen as $w ) {
					if ( 'On waitlist' === $w['status_id:name'] ) {
						dg_ag_api( 'Participant', 'update', array( 'where' => array( array( 'id', '=', (int) $w['id'] ) ), 'values' => array( 'status_id:name' => 'Registered' ) ) );
						dg_ag_mail_contact( (int) $w['contact_id'], 'Je kunt erbij zijn: ' . $e['titel'], "Er kwam een plek vrij. Je bent nu aangemeld voor \"{$e['titel']}\" (" . dg_ag_wanneer( $e ) . ").\n\n" . dg_ag_url( $e ) . "\n", $e );
						break;
					}
				}
			}
		}
	}
	return $weg;
}

add_action( 'admin_post_nopriv_dg_evenement_aanmelden', static fn() => auth_redirect() );
add_action(
	'admin_post_dg_evenement_aanmelden',
	static function () {
		$id = absint( $_POST['id'] ?? 0 );
		$e  = dg_ag_evenement( $id );
		if ( ! $e || ! check_admin_referer( 'dg_evenement_aanmelden_' . $id ) ) {
			wp_die( 'Dat lukte niet.', 'Fout', array( 'response' => 403 ) );
		}
		$u   = wp_get_current_user();
		$cid = dg_ag_contact( $u->ID );
		if ( ! $cid ) {
			dg_ag_terug( dg_ag_url( $e ), array( 'Aanmelden lukte niet (geen contact in CiviCRM).' ), true );
		}
		if ( ! empty( $_POST['af'] ) ) {
			dg_ag_afmelding( $e, $cid );
			dg_ag_terug( dg_ag_url( $e ), array( 'Je bent afgemeld.' ) );
		}
		$reden = dg_ag_aanmelden_kan( $e );
		if ( $reden ) {
			dg_ag_terug( dg_ag_url( $e ), array( $reden ), true );
		}
		$status = dg_ag_aanmelding( $e, $cid );
		dg_ag_bevestigingsmail( $e, $u->user_email, $u->display_name, $status );
		dg_ag_terug( dg_ag_url( $e ), array( 'On waitlist' === $status ? 'Het is vol: je staat op de wachtlijst. Komt er een plek vrij, dan krijg je een mail.' : 'Je bent aangemeld. Je krijgt een mail met een uitnodiging voor je agenda.' ) );
	}
);

// Gasten: naam, e-mail, organisatie → een mail met een link om te bevestigen.
add_action( 'admin_post_dg_evenement_gast', 'dg_ag_gast' );
add_action( 'admin_post_nopriv_dg_evenement_gast', 'dg_ag_gast' );
function dg_ag_gast(): void {
	$id = absint( $_POST['id'] ?? 0 );
	$e  = dg_ag_evenement( $id );
	if ( ! $e || ! wp_verify_nonce( (string) wp_unslash( $_POST['_wpnonce'] ?? '' ), 'dg_evenement_gast_' . $id ) ) {
		wp_die( 'Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.', 'Verlopen', array( 'response' => 403 ) );
	}
	$terug = dg_ag_url( $e ) . '#aanmelden';
	$naar  = static function ( string $tekst, bool $fout ) use ( $terug ): void {
		set_transient( 'dg_ag_gastmelding_' . md5( (string) ( $_SERVER['REMOTE_ADDR'] ?? '' ) ), array( $tekst, $fout ), 10 * MINUTE_IN_SECONDS ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
		wp_safe_redirect( $terug );
		exit;
	};
	if ( 'iedereen' !== $e['aanmelden'] || ! $e['openbaar'] || dg_ag_aanmelden_kan( $e ) ) {
		$naar( dg_ag_aanmelden_kan( $e ) ?: 'Aanmelden is alleen voor leden.', true );
	}
	// Een veld dat mensen niet zien: wie het invult, is een robot.
	if ( ! empty( $_POST['website'] ) ) {
		$naar( 'Bedankt. Kijk in je mail.', false );
	}
	$ip   = 'dg_ag_ip_' . md5( (string) ( $_SERVER['REMOTE_ADDR'] ?? '' ) ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
	$keer = (int) get_transient( $ip );
	if ( $keer >= 5 ) {
		$naar( 'Te vaak geprobeerd. Probeer het over een uur opnieuw.', true );
	}
	set_transient( $ip, $keer + 1, HOUR_IN_SECONDS );
	$naam  = mb_substr( trim( sanitize_text_field( wp_unslash( $_POST['naam'] ?? '' ) ) ), 0, 120 );
	$email = sanitize_email( wp_unslash( $_POST['email'] ?? '' ) );
	$org   = mb_substr( trim( sanitize_text_field( wp_unslash( $_POST['organisatie'] ?? '' ) ) ), 0, 120 );
	if ( '' === $naam || ! is_email( $email ) ) {
		$naar( 'Vul je naam en een geldig e-mailadres in.', true );
	}
	$sleutel = bin2hex( random_bytes( 24 ) );
	set_transient( 'dg_ag_gast_' . hash( 'sha256', $sleutel ), array( 'id' => $id, 'naam' => $naam, 'email' => $email, 'org' => $org ), DAY_IN_SECONDS );
	wp_mail(
		$email,
		'Bevestig je aanmelding: ' . $e['titel'],
		"Hallo $naam,\n\nJe wilt je aanmelden voor \"{$e['titel']}\" (" . dg_ag_wanneer( $e ) . ").\n\nBevestig met deze link (24 uur geldig):\n\n" . home_url( '/agenda/bevestig/' . $sleutel . '/' ) . "\n\nWas jij dit niet? Dan hoef je niets te doen; we bewaren je gegevens niet.\n\nDigitaal GemeenGoed\n"
	);
	$naar( 'Bijna klaar: we stuurden je een mail. Klik op de link daarin om je aanmelding te bevestigen.', false );
}

/** Een sleutel waarmee een gast zich zonder account afmeldt (in de bevestigingsmail). */
function dg_ag_gastsleutel( int $deelnemer ): string {
	return substr( hash_hmac( 'sha256', 'afmelden-' . $deelnemer, wp_salt( 'auth' ) ), 0, 24 );
}

// ---------------------------------------------------------------------------
// Mail en iCal
// ---------------------------------------------------------------------------

function dg_ag_wanneer( array $e ): string {
	$zelfde = wp_date( 'Ymd', $e['begin'] ) === wp_date( 'Ymd', $e['eind'] );
	return wp_date( 'l j F Y, H:i', $e['begin'] ) . ( $zelfde ? '–' . wp_date( 'H:i', $e['eind'] ) : ' tot ' . wp_date( 'l j F Y, H:i', $e['eind'] ) );
}

/** Waar: adres en/of online. */
function dg_ag_waar( array $e, bool $lang = true ): string {
	$plek = trim( $e['adres'] . ( $e['adres'] && $e['plaats'] ? ', ' : '' ) . $e['plaats'] );
	if ( ! $lang && $e['plaats'] ) {
		$plek = $e['plaats'];
	}
	if ( $e['online_link'] ) {
		$plek = $plek ? "$plek en online" : 'Online';
	}
	return $plek;
}

/** Mag de bezoeker de link naar de online meeting zien? */
function dg_ag_link_zichtbaar( array $e ): bool {
	if ( ! $e['online_link'] ) {
		return false;
	}
	if ( 'openbaar' === $e['link_zichtbaar'] ) {
		return true;
	}
	if ( ! is_user_logged_in() ) {
		return false;
	}
	if ( 'leden' === $e['link_zichtbaar'] || dg_ag_mag_bewerken( $e ) ) {
		return true;
	}
	return in_array( dg_ag_mijn_aanmeldingen()[ $e['id'] ] ?? '', DG_AG_TELT, true );
}

function dg_ag_ics_tekst( string $s ): string {
	return str_replace( array( '\\', ';', ',', "\r\n", "\n" ), array( '\\\\', '\\;', '\\,', '\\n', '\\n' ), $s );
}

function dg_ag_ics_regel( string $regel ): string {
	$uit = '';
	while ( strlen( $regel ) > 74 ) {
		$knip = 74;
		while ( $knip > 0 && ( ord( $regel[ $knip ] ) & 0xC0 ) === 0x80 ) {
			--$knip; // niet midden in een UTF-8-teken knippen
		}
		$uit  .= substr( $regel, 0, $knip ) . "\r\n ";
		$regel = substr( $regel, $knip );
	}
	return $uit . $regel . "\r\n";
}

/** Eén VEVENT; $met_link: de link naar de online meeting erin (als de ontvanger hem mag zien). */
function dg_ag_vevent( array $e, bool $met_link ): string {
	$utc   = static fn( int $t ) => gmdate( 'Ymd\THis\Z', $t );
	$tekst = implode( "\n\n", array_filter( array( $e['samenvatting'], trim( wp_strip_all_tags( $e['omschrijving'] ) ), 'extern' === $e['soort'] ? $e['extern_link'] : '', dg_ag_url( $e ) ) ) );
	if ( $met_link && $e['online_link'] ) {
		$tekst .= "\n\nOnline: " . $e['online_link'];
	}
	$plek = trim( $e['adres'] . ( $e['adres'] && $e['plaats'] ? ', ' : '' ) . $e['plaats'] ) ?: ( $met_link && $e['online_link'] ? $e['online_link'] : ( $e['online_link'] ? 'Online' : '' ) );
	$r    = array(
		'BEGIN:VEVENT',
		'UID:evenement-' . $e['id'] . '@' . wp_parse_url( home_url(), PHP_URL_HOST ),
		'DTSTAMP:' . $utc( time() ),
		'DTSTART:' . $utc( $e['begin'] ),
		'DTEND:' . $utc( $e['eind'] ),
		'SUMMARY:' . dg_ag_ics_tekst( ( $e['afgelast'] ? 'AFGELAST: ' : '' ) . $e['titel'] ),
		'DESCRIPTION:' . dg_ag_ics_tekst( $tekst ),
		'URL:' . dg_ag_url( $e ),
		'STATUS:' . ( $e['afgelast'] ? 'CANCELLED' : 'CONFIRMED' ),
	);
	if ( $plek ) {
		$r[] = 'LOCATION:' . dg_ag_ics_tekst( $plek );
	}
	$r[] = 'END:VEVENT';
	return implode( '', array_map( 'dg_ag_ics_regel', $r ) );
}

function dg_ag_vcalendar( string $naam, string $events ): string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Digitaal GemeenGoed//Agenda//NL\r\nCALSCALE:GREGORIAN\r\nMETHOD:PUBLISH\r\n" . dg_ag_ics_regel( 'X-WR-CALNAME:' . dg_ag_ics_tekst( $naam ) ) . $events . "END:VCALENDAR\r\n";
}

/** Een mail aan een contact in CiviCRM (op het eerste e-mailadres). */
function dg_ag_mail_contact( int $contact, string $onderwerp, string $tekst, ?array $e = null ): bool {
	$m = dg_ag_api( 'Email', 'get', array( 'select' => array( 'email' ), 'where' => array( array( 'contact_id', '=', $contact ) ), 'orderBy' => array( 'is_primary' => 'DESC' ), 'limit' => 1 ) );
	if ( ! $m ) {
		return false;
	}
	return dg_ag_mail( (string) $m[0]['email'], $onderwerp, $tekst, $e );
}

/** Een mail, met het evenement als .ics erbij (om in je agenda te zetten). */
function dg_ag_mail( string $aan, string $onderwerp, string $tekst, ?array $e = null, bool $met_link = true ): bool {
	$bijlage = array();
	if ( $e ) {
		$pad = trailingslashit( get_temp_dir() ) . 'evenement-' . $e['id'] . '-' . wp_generate_password( 6, false ) . '.ics';
		file_put_contents( $pad, dg_ag_vcalendar( $e['titel'], dg_ag_vevent( $e, $met_link ) ) ); // phpcs:ignore WordPress.WP.AlternativeFunctions
		$bijlage[] = $pad;
	}
	$ok = (bool) wp_mail( $aan, $onderwerp, $tekst . "\nDigitaal GemeenGoed\n" . home_url( '/' ) . "\n", array( 'Content-Type: text/plain; charset=UTF-8' ), $bijlage );
	foreach ( $bijlage as $b ) {
		wp_delete_file( $b );
	}
	return $ok;
}

function dg_ag_bevestigingsmail( array $e, string $email, string $naam, string $status, string $afmelden = '' ): void {
	$link  = $e['online_link'] && 'On waitlist' !== $status ? "\n\nOnline: " . $e['online_link'] : '';
	$tekst = "Hallo $naam,\n\n" . ( 'On waitlist' === $status
		? "Het is vol bij \"{$e['titel']}\". Je staat op de wachtlijst; komt er een plek vrij, dan krijg je een mail."
		: "Je bent aangemeld voor \"{$e['titel']}\".\n\n" . dg_ag_wanneer( $e ) . ( dg_ag_waar( $e ) ? "\n" . dg_ag_waar( $e ) : '' ) . $link . "\n\nDe uitnodiging voor je agenda zit bij deze mail." )
		. "\n\nMeer informatie: " . dg_ag_url( $e ) . "\n"
		. ( $afmelden ? "Toch niet? Meld je af: $afmelden\n" : "Toch niet? Meld je af op de pagina van het evenement.\n" );
	dg_ag_mail( $email, ( 'On waitlist' === $status ? 'Wachtlijst: ' : 'Aangemeld: ' ) . $e['titel'], $tekst, 'On waitlist' === $status ? null : $e );
}

// ---------------------------------------------------------------------------
// De pagina's
// ---------------------------------------------------------------------------

add_filter(
	'document_title_parts',
	static function ( $delen ) {
		$wat = get_query_var( 'dg_agenda' );
		if ( ! $wat ) {
			return $delen;
		}
		$e              = in_array( $wat, array( 'evenement', 'bewerken' ), true ) ? dg_ag_evenement( (int) get_query_var( 'dg_ag_id' ) ) : null;
		$delen['title'] = $e && dg_ag_mag_zien( $e ) ? $e['titel'] . ' · Agenda' : ( 'nieuw' === $wat ? 'Evenement toevoegen · Agenda' : 'Agenda' );
		return $delen;
	}
);

add_filter(
	'body_class',
	static function ( $k ) {
		return get_query_var( 'dg_agenda' ) ? array_merge( array_diff( $k, array( 'home', 'blog', 'error404' ) ), array( 'dg-agenda-pagina', 'dg-berichten-pagina' ) ) : $k;
	}
);

add_action(
	'wp_enqueue_scripts',
	static function () {
		if ( get_query_var( 'dg_agenda' ) ) {
			$v = wp_get_theme()->get( 'Version' ) . '.' . DG_AG_VERSIE;
			wp_enqueue_style( 'dg-agenda', get_stylesheet_directory_uri() . '/agenda.css', array(), $v );
			wp_enqueue_script( 'dg-agenda', get_stylesheet_directory_uri() . '/agenda.js', array(), $v, array( 'in_footer' => true, 'strategy' => 'defer' ) );
		}
	},
	30
);

// Het kaartje bij het delen van een openbaar evenement.
add_action(
	'wp_head',
	static function () {
		if ( 'evenement' !== get_query_var( 'dg_agenda' ) ) {
			return;
		}
		$e = dg_ag_evenement( (int) get_query_var( 'dg_ag_id' ) );
		if ( ! $e || ! $e['openbaar'] ) {
			echo '<meta name="robots" content="noindex">' . "\n";
			return;
		}
		$meta = array(
			'og:type'        => 'website',
			'og:site_name'   => 'Digitaal GemeenGoed',
			'og:title'       => $e['titel'],
			'og:description' => dg_ag_wanneer( $e ) . ( dg_ag_waar( $e ) ? ' · ' . dg_ag_waar( $e ) : '' ) . ( $e['samenvatting'] ? ' — ' . $e['samenvatting'] : '' ),
			'og:url'         => dg_ag_url( $e ),
			'og:image'       => $e['afbeelding'] ? (string) wp_get_attachment_image_url( $e['afbeelding'], 'large' ) : '',
		);
		foreach ( array_filter( $meta ) as $k => $v ) {
			printf( '<meta property="%s" content="%s">' . "\n", esc_attr( $k ), esc_attr( $v ) );
		}
	},
	5
);

add_action(
	'template_redirect',
	static function () {
		$wat = (string) get_query_var( 'dg_agenda' );
		if ( '' === $wat ) {
			return;
		}
		$id = (int) get_query_var( 'dg_ag_id' );
		switch ( $wat ) {
			case 'ical':
				dg_ag_ical( (string) get_query_var( 'dg_ag_sleutel' ) );
				exit;
			case 'ics':
				$e = dg_ag_evenement( $id );
				if ( ! $e || ! dg_ag_mag_zien( $e ) ) {
					is_user_logged_in() ? wp_die( 'Niet gevonden.', 'Agenda', array( 'response' => 404 ) ) : auth_redirect();
				}
				nocache_headers();
				header( 'Content-Type: text/calendar; charset=utf-8' );
				header( 'Content-Disposition: attachment; filename="' . sanitize_file_name( $e['titel'] ) . '.ics"' );
				echo dg_ag_vcalendar( $e['titel'], dg_ag_vevent( $e, dg_ag_link_zichtbaar( $e ) ) ); // phpcs:ignore WordPress.Security.EscapeOutput
				exit;
			case 'csv':
				dg_ag_csv( $id );
				exit;
			case 'bevestig':
				dg_ag_bevestig( (string) get_query_var( 'dg_ag_sleutel' ) );
				exit;
			case 'gastafmelden':
				dg_ag_gast_afmelden( $id, (string) get_query_var( 'dg_ag_sleutel' ) );
				exit;
			case 'nieuw':
				if ( ! is_user_logged_in() ) {
					auth_redirect();
				}
				break;
			case 'evenement':
			case 'bewerken':
				$e = dg_ag_evenement( $id );
				if ( $e && ! dg_ag_mag_zien( $e ) ) {
					auth_redirect();
				}
				if ( 'bewerken' === $wat && ( ! $e || ! dg_ag_mag_bewerken( $e ) ) ) {
					wp_safe_redirect( $e ? dg_ag_url( $e ) : dg_ag_url() );
					exit;
				}
				if ( ! $e ) {
					status_header( 404 );
				}
				break;
		}
		remove_action( 'buddyx_sub_header', 'buddyx_sub_header' );
		get_header();
		echo dg_ag_pagina( $wat, $id ); // phpcs:ignore WordPress.Security.EscapeOutput -- opgebouwd met esc_*.
		get_footer();
		exit;
	},
	20
);

function dg_ag_pagina( string $wat, int $id ): string {
	$h  = '<div class="dg-ber-pagina dg-ag-pagina"><main class="dg-ber-hoofd" id="dg-agenda">';
	$h .= function_exists( 'dg_ber_melding' ) ? dg_ber_melding() : '';
	if ( ! dg_ag_civi() ) {
		$h .= '<div class="dg-ber-feedkop"><h1>Agenda</h1><p>De agenda is nu niet bereikbaar. Probeer het later opnieuw.</p></div>';
	} elseif ( 'nieuw' === $wat ) {
		$h .= '<p class="dg-ber-terug"><a href="' . esc_url( dg_ag_url() ) . '">‹ Agenda</a></p><h1 class="dg-ber-kop">Evenement toevoegen</h1>' . dg_ag_formulier( null );
	} elseif ( 'bewerken' === $wat ) {
		$e  = dg_ag_evenement( $id );
		$h .= '<p class="dg-ber-terug"><a href="' . esc_url( dg_ag_url( $e ) ) . '">‹ Terug naar het evenement</a></p><h1 class="dg-ber-kop">Evenement bewerken</h1>' . dg_ag_formulier( $e );
	} elseif ( 'evenement' === $wat ) {
		$e  = dg_ag_evenement( $id );
		$h .= '<p class="dg-ber-terug"><a href="' . esc_url( dg_ag_url() ) . '">‹ Agenda</a></p>';
		$h .= $e ? dg_ag_detail( $e ) : '<div class="dg-ber-feedkop"><h1>Niet gevonden</h1><p>Dit evenement bestaat niet (meer).</p></div>';
	} else {
		$h .= dg_ag_lijst();
	}
	$h .= '</main><aside class="dg-ber-zij">' . dg_ag_zijbalk() . '</aside></div>';
	return $h;
}

/** De filters van de lijst, uit de URL. */
function dg_ag_filter(): array {
	// phpcs:disable WordPress.Security.NonceVerification.Recommended -- alleen filteren.
	return array(
		'tag'    => sanitize_title( wp_unslash( $_GET['tag'] ?? '' ) ),
		'soort'  => 'extern' === ( $_GET['soort'] ?? '' ) ? 'extern' : ( 'eigen' === ( $_GET['soort'] ?? '' ) ? 'eigen' : '' ),
		'mijn'   => ! empty( $_GET['mijn'] ) && is_user_logged_in(),
		'eerder' => ! empty( $_GET['eerder'] ),
	);
	// phpcs:enable
}

function dg_ag_lijst(): string {
	$f    = dg_ag_filter();
	$tags = dg_ag_vaste_tags();
	$h    = '<header class="dg-ber-feedkop dg-ag-kop"><div><h1>' . ( $f['tag'] ? 'Agenda: ' . esc_html( dg_ag_tagnaam( $f['tag'] ) ) : 'Agenda' ) . '</h1>'
		. '<p>' . ( $f['eerder'] ? 'Eerdere evenementen.' : 'Bijeenkomsten van de coöperatie, werkgroepen en projecten, en tips van leden voor evenementen elders.' ) . '</p></div>'
		. ( is_user_logged_in() ? '<a class="dg-db-knop" href="' . esc_url( add_query_arg( 'tag', $f['tag'] ?: false, dg_ag_url( null, 'nieuw' ) ) ) . '">+ Evenement toevoegen</a>' : '' ) . '</header>';

	// Filters.
	$basis = remove_query_arg( array( 'tag', 'soort', 'mijn', 'eerder' ) );
	$chip  = static function ( string $label, array $args, bool $aan ) use ( $basis ): string {
		return '<a class="dg-ag-chip' . ( $aan ? ' is-aan' : '' ) . '" href="' . esc_url( add_query_arg( $args, $basis ) ) . '"' . ( $aan ? ' aria-current="true"' : '' ) . '>' . esc_html( $label ) . '</a>';
	};
	$h .= '<nav class="dg-ag-filters" aria-label="Filters">'
		. $chip( 'Alles', array(), ! $f['tag'] && ! $f['soort'] && ! $f['mijn'] && ! $f['eerder'] )
		. $chip( 'Coöperatie', array( 'tag' => 'cooperatie' ), 'cooperatie' === $f['tag'] )
		. $chip( 'Van ons', array( 'soort' => 'eigen' ), 'eigen' === $f['soort'] )
		. $chip( 'Elders (tips)', array( 'soort' => 'extern' ), 'extern' === $f['soort'] )
		. ( is_user_logged_in() ? $chip( 'Mijn aanmeldingen', array( 'mijn' => 1 ), $f['mijn'] ) : '' )
		. '<form method="get" action="' . esc_url( dg_ag_url() ) . '" class="dg-ag-tagkeuze"><label class="screen-reader-text" for="dg-ag-tag">Werkgroep of project</label><select id="dg-ag-tag" name="tag" data-direct><option value="">Werkgroep of project…</option>';
	foreach ( $tags as $slug => $naam ) {
		if ( 'cooperatie' !== $slug ) {
			$h .= '<option value="' . esc_attr( $slug ) . '"' . selected( $f['tag'], $slug, false ) . '>' . esc_html( dg_ag_tagsoort( $slug ) . ': ' . $naam ) . '</option>';
		}
	}
	$h .= '</select><noscript><button type="submit">Toon</button></noscript></form></nav>';

	$lijst = dg_ag_evenementen( $f );
	if ( ! $lijst ) {
		$h .= '<p class="dg-ber-leeg">' . ( $f['eerder'] ? 'Geen eerdere evenementen.' : 'Er staat nog niets in de agenda' . ( $f['tag'] || $f['soort'] || $f['mijn'] ? ' bij deze keuze' : '' ) . '.' ) . ( is_user_logged_in() && ! $f['eerder'] ? ' <a href="' . esc_url( dg_ag_url( null, 'nieuw' ) ) . '">Zet er een in</a>.' : '' ) . '</p>';
	} else {
		$tel  = dg_ag_deelnemers( array_column( $lijst, 'id' ) );
		$mijn = dg_ag_mijn_aanmeldingen();
		$maand = '';
		$h   .= '<div class="dg-ag-lijst">';
		foreach ( $lijst as $e ) {
			$m = wp_date( 'F Y', $e['begin'] );
			if ( $m !== $maand ) {
				$h    .= ( $maand ? '</ol>' : '' ) . '<h2 class="dg-ag-maand">' . esc_html( ucfirst( $m ) ) . '</h2><ol class="dg-ag-items">';
				$maand = $m;
			}
			$h .= dg_ag_item( $e, dg_ag_aantal( $tel[ $e['id'] ] ?? array() ), $mijn[ $e['id'] ] ?? '' );
		}
		$h .= '</ol></div>';
	}
	$h .= '<p class="dg-ag-eerder">' . ( $f['eerder'] ? '<a href="' . esc_url( remove_query_arg( 'eerder' ) ) . '">‹ Komende evenementen</a>' : '<a href="' . esc_url( add_query_arg( 'eerder', 1 ) ) . '">Eerdere evenementen ›</a>' ) . '</p>';
	return $h;
}

/** Eén evenement in de lijst. */
function dg_ag_item( array $e, int $aantal, string $mijn ): string {
	$labels = array();
	if ( $e['afgelast'] ) {
		$labels[] = '<span class="dg-ag-label is-afgelast">Afgelast</span>';
	}
	if ( 'extern' === $e['soort'] ) {
		$labels[] = '<span class="dg-ag-label is-extern">Elders</span>';
	}
	if ( in_array( $mijn, DG_AG_TELT, true ) ) {
		$labels[] = '<span class="dg-ag-label is-aangemeld">✓ Aangemeld</span>';
	} elseif ( 'On waitlist' === $mijn ) {
		$labels[] = '<span class="dg-ag-label">Wachtlijst</span>';
	}
	if ( ! $e['openbaar'] ) {
		$labels[] = '<span class="dg-ag-label"><span class="dg-icoon dg-icoon-slot" aria-hidden="true"></span>Leden</span>';
	}
	$zelfde = wp_date( 'Ymd', $e['begin'] ) === wp_date( 'Ymd', $e['eind'] );
	$tijd   = wp_date( 'H:i', $e['begin'] ) . '–' . ( $zelfde ? wp_date( 'H:i', $e['eind'] ) : wp_date( 'j M H:i', $e['eind'] ) );
	return '<li class="dg-ag-item' . ( $e['afgelast'] ? ' is-afgelast' : '' ) . '" id="evenement-' . (int) $e['id'] . '">'
		. '<span class="dg-ag-datum"><small>' . esc_html( wp_date( 'D', $e['begin'] ) ) . '</small><b>' . esc_html( wp_date( 'j', $e['begin'] ) ) . '</b><small>' . esc_html( wp_date( 'M', $e['begin'] ) ) . '</small></span>'
		. '<div class="dg-ag-info"><a class="dg-ag-titel" href="' . esc_url( dg_ag_url( $e ) ) . '">' . esc_html( $e['titel'] ) . '</a>'
		. ( $labels ? '<span class="dg-ag-labels">' . implode( '', $labels ) . '</span>' : '' )
		. '<p class="dg-ag-meta"><span><span class="dg-icoon dg-icoon-kalender" aria-hidden="true"></span>' . esc_html( $tijd ) . '</span>'
		. ( dg_ag_waar( $e, false ) ? '<span><span class="dg-icoon dg-icoon-kompas" aria-hidden="true"></span>' . esc_html( dg_ag_waar( $e, false ) ) . '</span>' : '' )
		. ( 'uit' !== $e['aanmelden'] ? '<span><span class="dg-icoon dg-icoon-mensen" aria-hidden="true"></span>' . (int) $aantal . ( $e['max'] ? ' / ' . (int) $e['max'] : '' ) . ' aangemeld</span>' : '' ) . '</p>'
		. ( $e['samenvatting'] ? '<p class="dg-ag-samen">' . esc_html( $e['samenvatting'] ) . '</p>' : '' )
		. ( $e['tag'] ? '<a class="dg-ber-tag is-vast" href="' . esc_url( add_query_arg( 'tag', $e['tag'], dg_ag_url() ) ) . '">' . esc_html( ( 'extern' === $e['soort'] ? '' : 'Door ' ) . dg_ag_tagnaam( $e['tag'] ) ) . '</a>' : '' )
		. '</div>'
		. ( $e['afbeelding'] ? '<a class="dg-ag-beeld" href="' . esc_url( dg_ag_url( $e ) ) . '" tabindex="-1" aria-hidden="true">' . wp_get_attachment_image( $e['afbeelding'], 'thumbnail', false, array( 'alt' => '', 'loading' => 'lazy' ) ) . '</a>' : '' )
		. '</li>';
}

/** De pagina van één evenement. */
function dg_ag_detail( array $e ): string {
	$rijen  = dg_ag_deelnemers( array( $e['id'] ), dg_ag_mag_bewerken( $e ) )[ $e['id'] ] ?? array();
	$aantal = dg_ag_aantal( $rijen );
	$mijn   = dg_ag_mijn_aanmeldingen()[ $e['id'] ] ?? '';
	$h      = '<article class="dg-ber-kaart dg-ag-detail' . ( $e['afgelast'] ? ' is-afgelast' : '' ) . '">';
	if ( $e['afbeelding'] ) {
		$h .= '<div class="dg-ag-held">' . wp_get_attachment_image( $e['afbeelding'], 'large', false, array( 'alt' => '' ) ) . '</div>';
	}
	$h .= '<div class="dg-ag-detailkop"><span class="dg-ag-datum is-groot"><small>' . esc_html( wp_date( 'D', $e['begin'] ) ) . '</small><b>' . esc_html( wp_date( 'j', $e['begin'] ) ) . '</b><small>' . esc_html( wp_date( 'M Y', $e['begin'] ) ) . '</small></span><div>';
	$h .= '<p class="dg-ag-boven">' . ( 'extern' === $e['soort'] ? 'Evenement elders' : ( $e['tag'] ? esc_html( dg_ag_tagsoort( $e['tag'] ) ) . ': ' . esc_html( dg_ag_tagnaam( $e['tag'] ) ) : 'Evenement' ) )
		. ' · ' . ( $e['openbaar'] ? '<span class="dg-icoon dg-icoon-wereld" aria-hidden="true"></span>Openbaar' : '<span class="dg-icoon dg-icoon-slot" aria-hidden="true"></span>Alleen leden' ) . '</p>';
	$h .= '<h1 class="dg-ber-titel">' . ( $e['afgelast'] ? '<span class="dg-ag-label is-afgelast">Afgelast</span> ' : '' ) . esc_html( $e['titel'] ) . '</h1>';
	if ( $e['samenvatting'] ) {
		$h .= '<p class="dg-ag-samen">' . esc_html( $e['samenvatting'] ) . '</p>';
	}
	$h .= '</div></div><dl class="dg-ag-feiten">';
	$h .= '<div><dt><span class="dg-icoon dg-icoon-kalender" aria-hidden="true"></span>Wanneer</dt><dd>' . esc_html( ucfirst( dg_ag_wanneer( $e ) ) ) . '<br><a href="' . esc_url( dg_ag_url( $e, 'ics' ) ) . '">In je agenda zetten (.ics)</a></dd></div>';
	$plek = trim( $e['adres'] . ( $e['adres'] && $e['plaats'] ? ', ' : '' ) . $e['plaats'] );
	if ( $plek ) {
		$h .= '<div><dt><span class="dg-icoon dg-icoon-kompas" aria-hidden="true"></span>Waar</dt><dd>' . esc_html( $plek ) . '<br><a href="' . esc_url( 'https://www.openstreetmap.org/search?query=' . rawurlencode( $plek ) ) . '" target="_blank" rel="noopener">Op de kaart</a></dd></div>';
	}
	if ( $e['online_link'] ) {
		$uitleg = array(
			'leden'      => 'De link zien alleen leden. <a href="' . esc_url( wp_login_url( dg_ag_url( $e ) ) ) . '">Log in</a>.',
			'deelnemers' => 'De link zie je als je je hebt aangemeld' . ( 'uit' === $e['aanmelden'] ? '' : '' ) . '.',
		);
		$h .= '<div><dt><span class="dg-icoon dg-icoon-video" aria-hidden="true"></span>Online</dt><dd>'
			. ( dg_ag_link_zichtbaar( $e ) ? '<a href="' . esc_url( $e['online_link'] ) . '" target="_blank" rel="noopener">' . esc_html( preg_replace( '#^https?://#', '', $e['online_link'] ) ) . '</a>' : ( $uitleg[ $e['link_zichtbaar'] ] ?? '' ) )
			. '</dd></div>';
	}
	if ( 'extern' === $e['soort'] && $e['extern_link'] ) {
		$h .= '<div><dt><span class="dg-icoon dg-icoon-link" aria-hidden="true"></span>Organisator</dt><dd><a href="' . esc_url( $e['extern_link'] ) . '" target="_blank" rel="noopener nofollow">' . esc_html( (string) wp_parse_url( $e['extern_link'], PHP_URL_HOST ) ) . '</a> <small>(aanmelden daar)</small></dd></div>';
	}
	if ( $e['auteur'] && is_user_logged_in() ) {
		$u = get_userdata( $e['auteur'] );
		if ( $u ) {
			$h .= '<div><dt><span class="dg-icoon dg-icoon-mensen" aria-hidden="true"></span>' . ( 'extern' === $e['soort'] ? 'Tip van' : 'Contact' ) . '</dt><dd>'
				. ( function_exists( 'bp_members_get_user_url' ) ? '<a href="' . esc_url( bp_members_get_user_url( $u->ID ) ) . '">' . esc_html( $u->display_name ) . '</a>' : esc_html( $u->display_name ) ) . '</dd></div>';
		}
	}
	$h .= '</dl>';
	if ( $e['omschrijving'] ) {
		$h .= '<div class="dg-ag-omschrijving">' . wp_kses_post( make_clickable( $e['omschrijving'] ) ) . '</div>';
	}
	$h .= dg_ag_aanmeldblok( $e, $aantal, $mijn );

	// Delen.
	$url = dg_ag_url( $e );
	$h  .= '<footer class="dg-ber-acties"><button type="button" class="dg-ber-actie" data-kopieer="' . esc_attr( $url ) . '"><span class="dg-icoon dg-icoon-link" aria-hidden="true"></span>Link kopiëren</button>'
		. '<a class="dg-ber-actie" href="' . esc_url( 'mailto:?subject=' . rawurlencode( $e['titel'] ) . '&body=' . rawurlencode( $e['titel'] . "\n" . dg_ag_wanneer( $e ) . "\n\n" . $url ), array( 'mailto' ) ) . '"><span class="dg-icoon dg-icoon-mail" aria-hidden="true"></span>E-mail</a>'
		. ( $e['openbaar'] ? '<a class="dg-ber-actie" href="' . esc_url( 'https://www.linkedin.com/sharing/share-offsite/?url=' . rawurlencode( $url ) ) . '" target="_blank" rel="noopener"><span class="dg-icoon dg-icoon-delen" aria-hidden="true"></span>LinkedIn</a>' : '' )
		. '</footer></article>';

	// Beheer: deelnemers, bewerken, afgelasten, verwijderen.
	if ( dg_ag_mag_bewerken( $e ) ) {
		$h .= dg_ag_beheerblok( $e, $rijen );
	}
	return $h;
}

function dg_ag_aanmeldblok( array $e, int $aantal, string $mijn ): string {
	if ( 'uit' === $e['aanmelden'] ) {
		return '';
	}
	$h      = '<section class="dg-ag-aanmelden" id="aanmelden"><h2>Aanmelden</h2>';
	$h     .= '<p class="dg-ag-teller"><strong>' . (int) $aantal . '</strong> aangemeld' . ( $e['max'] ? ' van de ' . (int) $e['max'] . ' plaatsen' . ( $aantal >= $e['max'] ? ' — vol, er is een wachtlijst' : '' ) : '' ) . '.</p>';
	$reden  = dg_ag_aanmelden_kan( $e );
	if ( is_user_logged_in() ) {
		$knop = static function ( string $tekst, bool $af, string $klasse ) use ( $e ): string {
			return '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">' . wp_nonce_field( 'dg_evenement_aanmelden_' . $e['id'], '_wpnonce', true, false )
				. '<input type="hidden" name="action" value="dg_evenement_aanmelden"><input type="hidden" name="id" value="' . (int) $e['id'] . '">'
				. ( $af ? '<input type="hidden" name="af" value="1">' : '' ) . '<button type="submit" class="' . esc_attr( $klasse ) . '">' . esc_html( $tekst ) . '</button></form>';
		};
		if ( in_array( $mijn, DG_AG_TELT, true ) ) {
			$h .= '<p class="dg-ag-status is-aangemeld">✓ Je bent aangemeld.</p>' . ( $e['begin'] > time() ? $knop( 'Afmelden', true, 'dg-db-knop dg-db-knop-rand' ) : '' );
		} elseif ( 'On waitlist' === $mijn ) {
			$h .= '<p class="dg-ag-status">Je staat op de wachtlijst. Komt er een plek vrij, dan krijg je een mail.</p>' . $knop( 'Van de wachtlijst af', true, 'dg-db-knop dg-db-knop-rand' );
		} elseif ( $reden ) {
			$h .= '<p class="dg-ag-status">' . esc_html( $reden ) . '</p>';
		} else {
			$vol = $e['max'] && $aantal >= $e['max'];
			$h  .= $knop( $vol ? 'Op de wachtlijst' : 'Aanmelden', false, 'dg-db-knop' ) . '<p class="dg-ber-klein">Met één klik, met de gegevens van je account. Je krijgt een mail met een uitnodiging voor je agenda.</p>';
		}
	} elseif ( $reden ) {
		$h .= '<p class="dg-ag-status">' . esc_html( $reden ) . '</p>';
	} elseif ( 'iedereen' === $e['aanmelden'] && $e['openbaar'] ) {
		$m = get_transient( 'dg_ag_gastmelding_' . md5( (string) ( $_SERVER['REMOTE_ADDR'] ?? '' ) ) ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
		if ( is_array( $m ) ) {
			delete_transient( 'dg_ag_gastmelding_' . md5( (string) ( $_SERVER['REMOTE_ADDR'] ?? '' ) ) ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
			$h .= '<div class="dg-cf-melding' . ( $m[1] ? ' dg-cf-melding-fout' : '' ) . '" role="status">' . esc_html( $m[0] ) . '</div>';
		}
		$h .= '<p><a href="' . esc_url( wp_login_url( dg_ag_url( $e ) ) ) . '">Log in</a> als je lid bent, of meld je hier aan:</p>'
			. '<form class="dg-ag-gast" method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">' . wp_nonce_field( 'dg_evenement_gast_' . $e['id'], '_wpnonce', true, false )
			. '<input type="hidden" name="action" value="dg_evenement_gast"><input type="hidden" name="id" value="' . (int) $e['id'] . '">'
			. '<label>Naam<input type="text" name="naam" required maxlength="120" autocomplete="name"></label>'
			. '<label>E-mailadres<input type="email" name="email" required autocomplete="email"></label>'
			. '<label>Organisatie <small>(optioneel)</small><input type="text" name="organisatie" maxlength="120" autocomplete="organization"></label>'
			. '<label class="dg-ag-robot" aria-hidden="true">Website<input type="text" name="website" tabindex="-1" autocomplete="off"></label>'
			. '<p class="dg-ber-klein">Je krijgt een mail om je aanmelding te bevestigen. We bewaren je naam, e-mailadres en organisatie in onze ledenadministratie (CiviCRM), voor dit evenement; zie de <a href="' . esc_url( home_url( '/privacy/' ) ) . '">privacyverklaring</a>.</p>'
			. '<button type="submit" class="dg-db-knop">Aanmelden</button></form>';
	} else {
		$h .= '<p>Aanmelden is voor leden. <a href="' . esc_url( wp_login_url( dg_ag_url( $e ) ) ) . '">Log in</a> of <a href="' . esc_url( home_url( '/lid-worden/' ) ) . '">word lid</a>.</p>';
	}
	return $h . '</section>';
}

function dg_ag_beheerblok( array $e, array $rijen ): string {
	$actie = static function ( string $wat, string $tekst, string $bevestig, string $extra = '' ) use ( $e ): string {
		return '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" data-bevestig="' . esc_attr( $bevestig ) . '">' . wp_nonce_field( 'dg_evenement_beheer_' . $e['id'], '_wpnonce', true, false )
			. '<input type="hidden" name="action" value="dg_evenement_beheer"><input type="hidden" name="id" value="' . (int) $e['id'] . '"><input type="hidden" name="wat" value="' . esc_attr( $wat ) . '">'
			. $extra . '<button type="submit" class="dg-db-knop dg-db-knop-rand">' . esc_html( $tekst ) . '</button></form>';
	};
	$h = '<section class="dg-ber-opmerkingen dg-ag-beheer"><h2>Beheer</h2><div class="dg-ag-knoppen"><a class="dg-db-knop" href="' . esc_url( dg_ag_url( $e, 'bewerken' ) ) . '">Bewerken</a>';
	if ( $e['afgelast'] ) {
		$h .= $actie( 'doorgaan', 'Gaat toch door', 'Het evenement weer laten doorgaan?' );
	} else {
		$h .= '<details class="dg-ag-afgelasten"><summary class="dg-db-knop dg-db-knop-rand">Afgelasten…</summary>'
			. $actie( 'afgelasten', 'Afgelasten en deelnemers mailen', 'Het evenement afgelasten? De deelnemers krijgen een mail.', '<label>Bericht aan de deelnemers <small>(optioneel)</small><textarea name="reden" rows="2" maxlength="1000"></textarea></label>' ) . '</details>';
	}
	if ( ! dg_ag_aantal( $rijen ) ) {
		$h .= $actie( 'verwijderen', 'Verwijderen', 'Het evenement verwijderen? Dat kan niet ongedaan worden gemaakt.' );
	}
	$h .= '</div>';
	if ( 'uit' !== $e['aanmelden'] ) {
		$actief = array_filter( $rijen, static fn( $d ) => 'Cancelled' !== $d['status_id:name'] );
		$h     .= '<h3>Deelnemers (' . count( $actief ) . ')</h3>';
		if ( $actief ) {
			$h .= '<ol class="dg-ag-deelnemers">';
			foreach ( $actief as $d ) {
				$h .= '<li>' . esc_html( (string) ( $d['contact_id.display_name'] ?? '' ) )
					. ( ! empty( $d['contact_id.employer_id.display_name'] ) ? ' <small>' . esc_html( (string) $d['contact_id.employer_id.display_name'] ) . '</small>' : '' )
					. ( 'On waitlist' === $d['status_id:name'] ? ' <span class="dg-ag-label">wachtlijst</span>' : '' ) . '</li>';
			}
			$h .= '</ol><p><a href="' . esc_url( dg_ag_url( $e, 'deelnemers.csv' ) ) . '">Deelnemers downloaden (CSV)</a> · e-mailadressen en meer staan in CiviCRM.</p>';
		} else {
			$h .= '<p>Nog niemand.</p>';
		}
	}
	return $h . '</section>';
}

/** Het formulier voor een nieuw of bestaand evenement. */
function dg_ag_formulier( ?array $e ): string {
	$uid   = get_current_user_id();
	$terug = get_transient( 'dg_ag_invoer_' . $uid );
	delete_transient( 'dg_ag_invoer_' . $uid );
	$terug = is_array( $terug ) ? $terug : array();
	$w     = static function ( string $k, string $standaard = '' ) use ( $terug ): string {
		return isset( $terug[ $k ] ) ? (string) $terug[ $k ] : $standaard;
	};
	// phpcs:ignore WordPress.Security.NonceVerification.Recommended -- alleen een voorkeuze.
	$voor   = sanitize_title( wp_unslash( $_GET['tag'] ?? '' ) );
	$tags   = dg_ag_vaste_tags();
	$namens = array_filter( $tags, static fn( $n, $slug ) => dg_ag_mag_namens( (string) $slug ) || ( $e && $e['tag'] === $slug ), ARRAY_FILTER_USE_BOTH );
	$soort  = $w( 'soort', $e ? $e['soort'] : ( $namens ? 'eigen' : 'extern' ) );
	$tag    = $w( 'tag', $e ? $e['tag'] : $voor );
	$open   = $w( 'zichtbaar', $e ? ( $e['openbaar'] ? 'openbaar' : 'leden' ) : 'openbaar' );
	$aanm   = $w( 'aanmelden', $e ? $e['aanmelden'] : 'leden' );
	$lz     = $w( 'link_zichtbaar', $e ? $e['link_zichtbaar'] : 'deelnemers' );
	$opt    = static fn( array $keuzes, string $huidig ) => implode( '', array_map( static fn( $k, $l ) => '<option value="' . esc_attr( (string) $k ) . '"' . selected( $huidig, (string) $k, false ) . '>' . esc_html( $l ) . '</option>', array_keys( $keuzes ), $keuzes ) );
	$h      = '<form class="dg-ber-opstellen is-open dg-ag-form" method="post" enctype="multipart/form-data" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" data-agenda>'
		. wp_nonce_field( 'dg_evenement_opslaan', '_wpnonce', true, false ) . '<input type="hidden" name="action" value="dg_evenement_opslaan">'
		. ( $e ? '<input type="hidden" name="id" value="' . (int) $e['id'] . '">' : '' );

	$h .= '<fieldset class="dg-ag-soort"><legend>Wat voor evenement?</legend>'
		. '<label class="dg-ag-keuze"><input type="radio" name="soort" value="eigen"' . checked( $soort, 'eigen', false ) . ( $namens ? '' : ' disabled' ) . '><span><strong>Van ons</strong>Namens de coöperatie, een werkgroep of een project' . ( $namens ? '' : ' <em>(alleen als je een werkgroep of project beheert, of in de kerngroep zit)</em>' ) . '</span></label>'
		. '<label class="dg-ag-keuze"><input type="radio" name="soort" value="extern"' . checked( $soort, 'extern', false ) . '><span><strong>Elders (tip)</strong>Een evenement van een andere organisatie dat interessant is voor leden</span></label></fieldset>';

	$met_soort = static function ( array $lijst ): array {
		$uit = array();
		foreach ( $lijst as $slug => $naam ) {
			$uit[ $slug ] = ( dg_ag_tagsoort( (string) $slug ) ? dg_ag_tagsoort( (string) $slug ) . ': ' : '' ) . $naam;
		}
		return $uit;
	};
	$h .= '<div class="dg-ag-alleen-eigen"><label>Namens <select name="tag">' . $opt( array( '' => 'Kies…' ) + $met_soort( $namens ), 'eigen' === $soort ? $tag : '' ) . '</select></label></div>';
	$h .= '<div class="dg-ag-alleen-extern"><label>Link naar de organisator <small>(daar meld je je aan)</small><input type="url" name="extern_link" inputmode="url" placeholder="https://…" value="' . esc_attr( $w( 'extern_link', $e ? $e['extern_link'] : '' ) ) . '"></label>'
		. '<label>Interessant voor <small>(optioneel: dan staat het ook in die agenda)</small><select name="tag_extern">' . $opt( array( '' => 'Iedereen' ) + $met_soort( $tags ), 'extern' === $soort ? $w( 'tag_extern', $tag ) : '' ) . '</select></label></div>';

	$h .= '<label>Titel<input type="text" name="titel" required maxlength="200" value="' . esc_attr( $w( 'titel', $e ? $e['titel'] : '' ) ) . '"></label>'
		. '<label>In één zin <small>(komt in de lijst en bij het delen)</small><input type="text" name="samenvatting" maxlength="300" value="' . esc_attr( $w( 'samenvatting', $e ? $e['samenvatting'] : '' ) ) . '"></label>'
		. '<label>Omschrijving <small>(programma, voor wie, wat mee te nemen)</small><textarea name="omschrijving" rows="6" maxlength="5000">' . esc_textarea( $w( 'omschrijving', $e ? trim( wp_strip_all_tags( str_replace( array( '</p>', '<br />', '<br>' ), array( "\n\n", "\n", "\n" ), $e['omschrijving'] ) ) ) : '' ) ) . '</textarea></label>';

	$h .= '<fieldset class="dg-ag-tijd"><legend>Wanneer</legend>'
		. '<label>Datum<input type="date" name="datum" required value="' . esc_attr( $w( 'datum', $e ? wp_date( 'Y-m-d', $e['begin'] ) : '' ) ) . '"></label>'
		. '<label>Van<input type="time" name="van" required value="' . esc_attr( $w( 'van', $e ? wp_date( 'H:i', $e['begin'] ) : '' ) ) . '"></label>'
		. '<label>Tot<input type="time" name="tot" value="' . esc_attr( $w( 'tot', $e ? wp_date( 'H:i', $e['eind'] ) : '' ) ) . '"></label>'
		. '<label>Tot en met <small>(meer dagen)</small><input type="date" name="einddatum" value="' . esc_attr( $w( 'einddatum', $e && wp_date( 'Ymd', $e['begin'] ) !== wp_date( 'Ymd', $e['eind'] ) ? wp_date( 'Y-m-d', $e['eind'] ) : '' ) ) . '"></label></fieldset>';

	$h .= '<fieldset class="dg-ag-waar"><legend>Waar</legend>'
		. '<label>Adres<input type="text" name="adres" maxlength="200" autocomplete="street-address" value="' . esc_attr( $w( 'adres', $e ? $e['adres'] : '' ) ) . '"></label>'
		. '<label>Plaats<input type="text" name="plaats" maxlength="100" value="' . esc_attr( $w( 'plaats', $e ? $e['plaats'] : '' ) ) . '"></label>'
		. '<label class="dg-ag-breed">Link naar de online meeting <small>(Teams, Jitsi, Zoom; optioneel)</small><input type="url" name="online_link" inputmode="url" placeholder="https://…" value="' . esc_attr( $w( 'online_link', $e ? $e['online_link'] : '' ) ) . '"></label>'
		. '<label class="dg-ag-breed">Wie ziet die link?<select name="link_zichtbaar">' . $opt( array( 'deelnemers' => 'Alleen wie zich aanmeldde (op uitnodiging)', 'leden' => 'Alle leden', 'openbaar' => 'Iedereen' ), $lz ) . '</select></label></fieldset>';

	$h .= '<fieldset class="dg-ag-wie"><legend>Wie</legend>'
		. '<label>Wie ziet het evenement?<select name="zichtbaar">' . $opt( array( 'openbaar' => '🌐 Iedereen (openbaar)', 'leden' => '🔒 Alleen leden' ), $open ) . '</select></label>'
		. '<label class="dg-ag-alleen-eigen">Aanmelden via de site<select name="aanmelden">' . $opt( array( 'leden' => 'Ja, voor leden', 'iedereen' => 'Ja, voor iedereen (ook gasten)', 'uit' => 'Nee' ), $aanm ) . '</select></label>'
		. '<label class="dg-ag-alleen-eigen">Hoeveel plaatsen? <small>(leeg: geen maximum; daarna een wachtlijst)</small><input type="number" name="max" min="1" max="10000" value="' . esc_attr( $w( 'max', $e && $e['max'] ? (string) $e['max'] : '' ) ) . '"></label></fieldset>';

	$h .= '<label>Afbeelding <small>(optioneel; voor de agenda en bij het delen)</small><input type="file" name="afbeelding" accept="image/jpeg,image/png,image/webp"></label>'
		. ( $e && $e['afbeelding'] ? '<p class="dg-ber-klein">Er is al een afbeelding; een nieuwe vervangt die.</p>' : '' )
		. ( ! $e && defined( 'DG_BER_TYPE' ) ? '<label class="dg-ag-vink"><input type="checkbox" name="nieuws" value="1"' . checked( $w( 'nieuws', '1' ), '1', false ) . '> Ook een bericht in het <strong>nieuws</strong> zetten</label>' : '' )
		. '<div class="dg-ber-opties"><button type="submit" class="dg-db-knop">' . ( $e ? 'Opslaan' : 'In de agenda zetten' ) . '</button><a href="' . esc_url( $e ? dg_ag_url( $e ) : dg_ag_url() ) . '">Annuleren</a></div>'
		. '<p class="dg-ber-klein">Geen persoonsgegevens of vertrouwelijke informatie in de omschrijving. Een evenement elders staat er meteen; wie het organiseert en ziet dat iets niet klopt, kan het melden bij de kerngroep.</p></form>';
	return $h;
}

/** Rechts: een maandoverzicht en de agenda in je eigen agenda (iCal). */
function dg_ag_zijbalk(): string {
	// phpcs:ignore WordPress.Security.NonceVerification.Recommended
	$m     = preg_match( '/^\d{4}-\d{2}$/', (string) ( $_GET['maand'] ?? '' ) ) ? (string) $_GET['maand'] : wp_date( 'Y-m' ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
	$eerste = new DateTimeImmutable( $m . '-01 00:00:00', wp_timezone() );
	$laatste = $eerste->modify( 'last day of this month' )->setTime( 23, 59, 59 );
	$f      = dg_ag_filter();
	$dagen  = array();
	foreach ( dg_ag_evenementen( array( 'van' => $eerste->format( 'Y-m-d H:i:s' ), 'tot' => $laatste->format( 'Y-m-d H:i:s' ), 'tag' => $f['tag'], 'soort' => $f['soort'] ) ) as $e ) {
		$d = (int) wp_date( 'j', $e['begin'] );
		if ( wp_date( 'Y-m', $e['begin'] ) === $m ) {
			$dagen[ $d ][] = $e;
		}
	}
	$vorige  = $eerste->modify( '-1 month' )->format( 'Y-m' );
	$volgend = $eerste->modify( '+1 month' )->format( 'Y-m' );
	$h       = '<section class="dg-ber-onderwerpen dg-ag-maandblok"><header><a href="' . esc_url( add_query_arg( 'maand', $vorige ) ) . '" aria-label="Vorige maand">‹</a><h2>' . esc_html( ucfirst( wp_date( 'F Y', $eerste->getTimestamp() ) ) ) . '</h2><a href="' . esc_url( add_query_arg( 'maand', $volgend ) ) . '" aria-label="Volgende maand">›</a></header>'
		. '<table class="dg-ag-maand-tabel"><thead><tr>';
	foreach ( array( 'ma', 'di', 'wo', 'do', 'vr', 'za', 'zo' ) as $d ) {
		$h .= '<th scope="col">' . $d . '</th>';
	}
	$h      .= '</tr></thead><tbody><tr>';
	$leeg    = (int) $eerste->format( 'N' ) - 1;
	$h      .= str_repeat( '<td></td>', $leeg );
	$vandaag = wp_date( 'Y-m-j' );
	for ( $d = 1, $kol = $leeg; $d <= (int) $laatste->format( 'j' ); $d++, $kol++ ) {
		if ( $kol && 0 === $kol % 7 ) {
			$h .= '</tr><tr>';
		}
		$klas = ( "$m-$d" === $vandaag ? ' is-vandaag' : '' ) . ( isset( $dagen[ $d ] ) ? ' heeft' : '' );
		$h   .= '<td class="' . trim( $klas ) . '">' . ( isset( $dagen[ $d ] )
			? '<a href="' . esc_url( dg_ag_url( $dagen[ $d ][0] ) ) . '" title="' . esc_attr( implode( "\n", array_map( static fn( $e ) => wp_date( 'H:i', $e['begin'] ) . ' ' . $e['titel'], $dagen[ $d ] ) ) ) . '">' . $d . '</a>'
			: '<span>' . $d . '</span>' ) . '</td>';
	}
	$h .= str_repeat( '<td></td>', ( 7 - $kol % 7 ) % 7 ) . '</tr></tbody></table></section>';

	// iCal-abonnement: openbaar, of met een persoonlijke sleutel ook de ledenevenementen.
	$q    = array_filter( array( 'tag' => $f['tag'], 'soort' => $f['soort'] ) );
	$open = add_query_arg( $q, dg_ag_url( null, 'ical' ) );
	$h   .= '<section class="dg-ber-onderwerpen dg-ag-abonneer"><h2><span class="dg-icoon dg-icoon-kalender" aria-hidden="true"></span>In je eigen agenda</h2>'
		. '<p>Abonneer je in Outlook, Google of Apple op deze agenda' . ( $f['tag'] ? ' (' . esc_html( dg_ag_tagnaam( $f['tag'] ) ) . ')' : '' ) . '. Nieuwe evenementen komen er dan vanzelf in.</p>';
	if ( is_user_logged_in() ) {
		$s    = (string) get_user_meta( get_current_user_id(), 'dg_ag_ical', true );
		if ( ! preg_match( '/^[a-f0-9]{32}$/', $s ) ) {
			$s = bin2hex( random_bytes( 16 ) );
			update_user_meta( get_current_user_id(), 'dg_ag_ical', $s );
		}
		$eigen = add_query_arg( $q, dg_ag_url( null, 'ical/' . $s ) );
		$h    .= '<p><button type="button" class="dg-db-knop" data-kopieer="' . esc_attr( $eigen ) . '">Link kopiëren</button> <a href="' . esc_url( str_replace( array( 'https://', 'http://' ), 'webcal://', $eigen ) ) . '">Openen</a></p>'
			. '<p class="dg-ber-klein">Deze link is persoonlijk (met de evenementen voor leden): deel hem niet. De openbare agenda: <button type="button" class="dg-ag-linkknop" data-kopieer="' . esc_attr( $open ) . '">link kopiëren</button>.</p>';
	} else {
		$h .= '<p><button type="button" class="dg-db-knop" data-kopieer="' . esc_attr( $open ) . '">Link kopiëren</button> <a href="' . esc_url( str_replace( array( 'https://', 'http://' ), 'webcal://', $open ) ) . '">Openen</a></p>';
	}
	return $h . '</section>';
}

/** De agenda als iCal: openbaar, of met de persoonlijke sleutel van een lid ook voor leden. */
function dg_ag_ical( string $sleutel ): void {
	$lid = 0;
	if ( $sleutel ) {
		$u   = get_users( array( 'meta_key' => 'dg_ag_ical', 'meta_value' => $sleutel, 'number' => 1, 'fields' => 'ID' ) ); // phpcs:ignore WordPress.DB.SlowDBQuery
		$lid = (int) ( $u[0] ?? 0 );
		if ( ! $lid ) {
			status_header( 404 );
			exit;
		}
		wp_set_current_user( $lid );
	}
	$f      = dg_ag_filter();
	$f['mijn'] = false;
	$events = '';
	// De komende evenementen en die van de laatste drie maanden.
	$alles = array_merge( dg_ag_evenementen( $f + array( 'limiet' => 300 ) ), dg_ag_evenementen( array( 'eerder' => true, 'limiet' => 50 ) + $f ) );
	foreach ( $alles as $e ) {
		if ( $e['eind'] > time() - 90 * DAY_IN_SECONDS ) {
			$events .= dg_ag_vevent( $e, $lid ? dg_ag_link_zichtbaar( $e ) : 'openbaar' === $e['link_zichtbaar'] );
		}
	}
	nocache_headers();
	header( 'Content-Type: text/calendar; charset=utf-8' );
	echo dg_ag_vcalendar( 'Digitaal GemeenGoed' . ( $f['tag'] ? ' · ' . dg_ag_tagnaam( $f['tag'] ) : '' ), $events ); // phpcs:ignore WordPress.Security.EscapeOutput
}

function dg_ag_csv( int $id ): void {
	$e = dg_ag_evenement( $id );
	if ( ! $e || ! is_user_logged_in() || ! dg_ag_mag_bewerken( $e ) ) {
		wp_die( 'Niet toegestaan.', 'Agenda', array( 'response' => 403 ) );
	}
	nocache_headers();
	header( 'Content-Type: text/csv; charset=utf-8' );
	header( 'Content-Disposition: attachment; filename="deelnemers-' . sanitize_file_name( $e['titel'] ) . '.csv"' );
	$uit = fopen( 'php://output', 'w' ); // phpcs:ignore WordPress.WP.AlternativeFunctions
	fwrite( $uit, "\xEF\xBB\xBF" ); // phpcs:ignore WordPress.WP.AlternativeFunctions -- BOM: Excel leest dan UTF-8
	fputcsv( $uit, array( 'Naam', 'Organisatie', 'Status', 'Aangemeld op' ), ';' );
	$status = array( 'Registered' => 'aangemeld', 'Attended' => 'aanwezig', 'On waitlist' => 'wachtlijst', 'Cancelled' => 'afgemeld' );
	foreach ( dg_ag_deelnemers( array( $id ), true )[ $id ] ?? array() as $d ) {
		$naam = (string) ( $d['contact_id.display_name'] ?? '' );
		// Geen formules in Excel uit wat mensen zelf invulden.
		$schoon = static fn( string $s ) => preg_match( '/^[=+\-@]/', $s ) ? "'" . $s : $s;
		fputcsv( $uit, array( $schoon( $naam ), $schoon( (string) ( $d['contact_id.employer_id.display_name'] ?? '' ) ), $status[ $d['status_id:name'] ] ?? $d['status_id:name'], (string) ( $d['register_date'] ?? '' ) ), ';' );
	}
	fclose( $uit ); // phpcs:ignore WordPress.WP.AlternativeFunctions
}

/** De link in de bevestigingsmail van een gast. */
function dg_ag_bevestig( string $sleutel ): void {
	$gast = get_transient( 'dg_ag_gast_' . hash( 'sha256', $sleutel ) );
	$e    = is_array( $gast ) ? dg_ag_evenement( (int) $gast['id'] ) : null;
	if ( ! $e ) {
		wp_die( 'Deze link is niet (meer) geldig. Een bevestigingslink is 24 uur geldig; meld je opnieuw aan.', 'Agenda', array( 'response' => 410 ) );
	}
	delete_transient( 'dg_ag_gast_' . hash( 'sha256', $sleutel ) );
	$reden = dg_ag_aanmelden_kan( $e );
	if ( $reden ) {
		wp_die( esc_html( $reden ), 'Agenda', array( 'response' => 409 ) );
	}
	try {
		$cid    = dg_ag_gast_contact( (string) $gast['email'], (string) $gast['naam'], (string) $gast['org'] );
		$status = dg_ag_aanmelding( $e, $cid );
		$pid    = dg_ag_api( 'Participant', 'get', array( 'select' => array( 'id' ), 'where' => array( array( 'event_id', '=', $e['id'] ), array( 'contact_id', '=', $cid ) ), 'limit' => 1 ) );
		$afm    = $pid ? home_url( '/agenda/afmelden/' . (int) $pid[0]['id'] . '-' . dg_ag_gastsleutel( (int) $pid[0]['id'] ) . '/' ) : '';
		dg_ag_bevestigingsmail( $e, (string) $gast['email'], (string) $gast['naam'], $status, $afm );
	} catch ( Throwable $f ) {
		error_log( 'dg-agenda: gast bevestigen: ' . $f->getMessage() );
		wp_die( 'Dat lukte niet. Probeer het later opnieuw.', 'Agenda', array( 'response' => 500 ) );
	}
	set_transient( 'dg_ag_gastmelding_' . md5( (string) ( $_SERVER['REMOTE_ADDR'] ?? '' ) ), array( 'On waitlist' === $status ? 'Bevestigd: het is vol, je staat op de wachtlijst.' : 'Bevestigd: je bent aangemeld. Je krijgt een mail met de uitnodiging voor je agenda.', false ), 10 * MINUTE_IN_SECONDS ); // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
	wp_safe_redirect( dg_ag_url( $e ) . '#aanmelden' );
	exit;
}

function dg_ag_gast_afmelden( int $pid, string $sleutel ): void {
	if ( ! hash_equals( dg_ag_gastsleutel( $pid ), $sleutel ) || ! dg_ag_civi() ) {
		wp_die( 'Deze link is niet geldig.', 'Agenda', array( 'response' => 404 ) );
	}
	$d = dg_ag_api( 'Participant', 'get', array( 'select' => array( 'event_id', 'contact_id' ), 'where' => array( array( 'id', '=', $pid ) ) ) );
	$e = $d ? dg_ag_evenement( (int) $d[0]['event_id'] ) : null;
	if ( ! $e ) {
		wp_die( 'Dit evenement bestaat niet meer.', 'Agenda', array( 'response' => 404 ) );
	}
	// phpcs:ignore WordPress.Security.NonceVerification.Missing -- de sleutel is de controle; POST om voorvertoners van mail niet te laten afmelden.
	if ( 'POST' !== ( $_SERVER['REQUEST_METHOD'] ?? '' ) ) {
		wp_die( '<p>Afmelden voor "' . esc_html( $e['titel'] ) . '" (' . esc_html( dg_ag_wanneer( $e ) ) . ')?</p><form method="post"><button type="submit">Ja, afmelden</button></form>', 'Afmelden', array( 'response' => 200 ) );
	}
	dg_ag_afmelding( $e, (int) $d[0]['contact_id'] );
	wp_die( '<p>Je bent afgemeld voor "' . esc_html( $e['titel'] ) . '".</p><p><a href="' . esc_url( dg_ag_url( $e ) ) . '">Naar het evenement</a></p>', 'Afgemeld', array( 'response' => 200 ) );
}

// ---------------------------------------------------------------------------
// Voor het dashboard en de homepage
// ---------------------------------------------------------------------------

/** De eerstvolgende evenementen met het aantal aanmeldingen, voor het dashboard. */
function dg_ag_volgende( int $n = 3 ): array {
	$lijst = array_slice( array_values( array_filter( dg_ag_evenementen( array( 'limiet' => $n + 5 ) ), static fn( $e ) => ! $e['afgelast'] ) ), 0, $n );
	$tel   = dg_ag_deelnemers( array_column( $lijst, 'id' ) );
	foreach ( $lijst as &$e ) {
		$e['aantal'] = dg_ag_aantal( $tel[ $e['id'] ] ?? array() );
		$e['url']    = dg_ag_url( $e );
	}
	return $lijst;
}
